package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/media"
	"github.com/andi-frame/lockedin/apps/server/internal/storage"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// MediaQueue is the media:process queue as the service sees it. Enqueue must be idempotent per
// attachment (a second call for a queued attachment succeeds), because the sweep calls it again
// for uploads whose first enqueue was lost.
type MediaQueue interface {
	Enqueue(ctx context.Context, attachment uuid.UUID) error
	// Busy reports that the backlog is at its limit, so new uploads should wait (ARCHITECTURE §6).
	Busy(ctx context.Context) (bool, error)
}

// UploadLimits are SPEC §8's raw limits. The media processor enforces the pixel and duration
// limits itself, after sniffing the real content.
type UploadLimits struct {
	ImageMaxBytes  int64
	VideoMaxBytes  int64
	FileMaxBytes   int64
	PactQuotaBytes int64
}

// UploadDeps are what the upload use-cases need. The API sets Blobs, Queue and Limits; the
// worker sets Blobs, Media and TmpDir.
type UploadDeps struct {
	Blobs  storage.BlobStore
	Queue  MediaQueue
	Media  *media.Processor
	Limits UploadLimits
	TmpDir string // where processing downloads files; empty means the OS temp dir
	Log    *slog.Logger
}

// WithUploads turns the upload use-cases on. Without it they answer ErrUploadsOff.
func (s *Service) WithUploads(d UploadDeps) *Service {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	s.up = &d
	return s
}

const (
	uploadURLTTL    = 10 * time.Minute // ARCHITECTURE §6: the presigned PUT
	viewURLTTL      = 5 * time.Minute  // the signed GET URLs of getAttachment
	staleUploadAge  = 24 * time.Hour
	orphanMediaAge  = 7 * 24 * time.Hour
	requeueAfter    = 5 * time.Minute
	gcBatch         = 100
	gcMaxBatches    = 10
	rejectSizeMatch = "Ukuran file yang terunggah tidak sama dengan yang dinyatakan. Coba unggah lagi."
)

// Accepted declared types per kind, SPEC §8. They only gate the intent; the real type is sniffed
// from the bytes by the media processor.
var allowedMIME = map[string]map[string]bool{
	"image": {"image/jpeg": true, "image/png": true, "image/webp": true, "image/heic": true, "image/heif": true, "image/avif": true, "image/gif": true},
	"video": {"video/mp4": true, "video/quicktime": true, "video/webm": true},
	"file":  {"application/pdf": true},
}

// UploadInput is the createUpload request.
type UploadInput struct {
	PactID uuid.UUID
	Kind   string
	Mime   string
	Bytes  int64
}

// UploadIntent is the presigned slot returned to the browser.
type UploadIntent struct {
	AttachmentID uuid.UUID
	PutURL       string
	Headers      map[string]string
	ExpiresAt    time.Time
}

// AttachmentURLs are signed GET URLs for a ready attachment.
type AttachmentURLs struct {
	Original  string
	Thumb     *string
	Poster    *string
	ExpiresAt time.Time
}

// AttachmentView is an attachment plus its URLs (only when ready).
type AttachmentView struct {
	store.Attachment
	URLs *AttachmentURLs
}

func (s *Service) uploadsOn() (*UploadDeps, error) {
	if s.up == nil || s.up.Blobs == nil {
		return nil, ErrUploadsOff
	}
	return s.up, nil
}

func (u *UploadDeps) maxBytes(kind string) int64 {
	switch kind {
	case "image":
		return u.Limits.ImageMaxBytes
	case "video":
		return u.Limits.VideoMaxBytes
	}
	return u.Limits.FileMaxBytes
}

// uploadablePact returns the pact when the user is a member and it can still take proof.
// Non-members and unknown pacts both get ErrNotFound (invariant 8).
func (s *Service) uploadablePact(ctx context.Context, user, pactID uuid.UUID) (store.Pact, error) {
	if _, err := s.st.GetPactMember(ctx, store.GetPactMemberParams{PactID: pactID, UserID: user}); store.IsNoRows(err) {
		return store.Pact{}, ErrNotFound
	} else if err != nil {
		return store.Pact{}, err
	}
	p, err := s.st.GetPact(ctx, pactID)
	if err != nil {
		return store.Pact{}, err
	}
	if p.Status != "active" && p.Status != "settling" {
		return store.Pact{}, ErrPactState
	}
	return p, nil
}

// CreateUpload checks the declared file against SPEC §8 and the pact quota, records an
// awaiting_upload attachment, and returns a presigned PUT. The bytes never touch the API.
func (s *Service) CreateUpload(ctx context.Context, user uuid.UUID, in UploadInput) (UploadIntent, error) {
	u, err := s.uploadsOn()
	if err != nil {
		return UploadIntent{}, err
	}
	if !allowedMIME[in.Kind][in.Mime] || in.Bytes <= 0 {
		return UploadIntent{}, ErrUploadUnsupported
	}
	if in.Bytes > u.maxBytes(in.Kind) {
		return UploadIntent{}, ErrUploadTooLarge
	}
	if _, err := s.uploadablePact(ctx, user, in.PactID); err != nil {
		return UploadIntent{}, err
	}
	if busy, err := u.Queue.Busy(ctx); err != nil {
		return UploadIntent{}, err
	} else if busy {
		return UploadIntent{}, ErrUploadQueueBusy
	}
	used, err := s.st.SumPactAttachmentBytes(ctx, in.PactID)
	if err != nil {
		return UploadIntent{}, err
	}
	if used+in.Bytes > u.Limits.PactQuotaBytes {
		return UploadIntent{}, ErrUploadQuota
	}

	id := newID()
	key := in.PactID.String() + "/" + id.String()
	if _, err := s.st.CreateAttachment(ctx, store.CreateAttachmentParams{
		ID: id, OwnerID: user, PactID: in.PactID, Kind: in.Kind,
		DeclaredMime: &in.Mime, DeclaredBytes: &in.Bytes, StagingKey: &key, CreatedAt: s.clock.Now(),
	}); err != nil {
		return UploadIntent{}, err
	}
	put, err := u.Blobs.PresignPut(ctx, storage.Staging, key, in.Bytes, in.Mime, uploadURLTTL)
	if err != nil {
		return UploadIntent{}, fmt.Errorf("presign upload: %w", err)
	}
	return UploadIntent{AttachmentID: id, PutURL: put.URL, Headers: put.Headers, ExpiresAt: put.Expires}, nil
}

// CompleteUpload runs when the browser's PUT finished: it checks the object's size against the
// declaration (defence in depth behind the signed Content-Length) and queues processing. Calling
// it again is harmless.
func (s *Service) CompleteUpload(ctx context.Context, user, id uuid.UUID) (store.Attachment, error) {
	u, err := s.uploadsOn()
	if err != nil {
		return store.Attachment{}, err
	}
	a, err := s.st.GetAttachment(ctx, id)
	if store.IsNoRows(err) || (err == nil && a.OwnerID != user) {
		return store.Attachment{}, ErrNotFound
	}
	if err != nil {
		return store.Attachment{}, err
	}
	if a.Status != "awaiting_upload" {
		return a, nil // a repeat call, or already further along
	}
	if _, err := s.uploadablePact(ctx, user, a.PactID); err != nil {
		return store.Attachment{}, err
	}
	if a.StagingKey == nil || a.DeclaredBytes == nil {
		return store.Attachment{}, ErrNotFound
	}

	info, err := u.Blobs.Head(ctx, storage.Staging, *a.StagingKey)
	if errors.Is(err, storage.ErrNotFound) {
		return store.Attachment{}, ErrUploadSizeMismatch // nothing arrived; the slot stays usable
	}
	if err != nil {
		return store.Attachment{}, err
	}
	if info.Size != *a.DeclaredBytes {
		_ = u.Blobs.Delete(ctx, storage.Staging, *a.StagingKey)
		if err := s.st.RejectAttachment(ctx, store.RejectAttachmentParams{ID: id, RejectReason: ptr(rejectSizeMatch)}); err != nil {
			return store.Attachment{}, err
		}
		return store.Attachment{}, ErrUploadSizeMismatch
	}
	if busy, err := u.Queue.Busy(ctx); err != nil {
		return store.Attachment{}, err
	} else if busy {
		return store.Attachment{}, ErrUploadQueueBusy
	}

	row, err := s.st.MarkAttachmentUploaded(ctx, id)
	if store.IsNoRows(err) { // a concurrent call won
		return s.st.GetAttachment(ctx, id)
	}
	if err != nil {
		return store.Attachment{}, err
	}
	// After the status change has committed (ARCHITECTURE §3). If this fails the row stays
	// 'uploaded' and CollectUploads queues it again.
	if err := u.Queue.Enqueue(ctx, id); err != nil {
		u.Log.Error("queue media processing", "attachment", id, "err", err)
	}
	return row, nil
}

// GetAttachment returns an attachment to the members of its pact, with signed URLs when ready.
func (s *Service) GetAttachment(ctx context.Context, user, id uuid.UUID) (AttachmentView, error) {
	u, err := s.uploadsOn()
	if err != nil {
		return AttachmentView{}, err
	}
	a, err := s.st.GetAttachment(ctx, id)
	if store.IsNoRows(err) {
		return AttachmentView{}, ErrNotFound
	}
	if err != nil {
		return AttachmentView{}, err
	}
	if _, err := s.st.GetPactMember(ctx, store.GetPactMemberParams{PactID: a.PactID, UserID: user}); store.IsNoRows(err) {
		return AttachmentView{}, ErrNotFound
	} else if err != nil {
		return AttachmentView{}, err
	}
	v := AttachmentView{Attachment: a}
	if a.Status != "ready" || a.MediaKey == nil {
		return v, nil
	}
	urls := &AttachmentURLs{ExpiresAt: s.clock.Now().Add(viewURLTTL)}
	if urls.Original, err = u.Blobs.PresignGet(ctx, storage.Media, *a.MediaKey, viewURLTTL); err != nil {
		return AttachmentView{}, err
	}
	if a.ThumbKey != nil {
		t, err := u.Blobs.PresignGet(ctx, storage.Media, *a.ThumbKey, viewURLTTL)
		if err != nil {
			return AttachmentView{}, err
		}
		if a.Kind == "video" {
			urls.Poster = &t
		} else {
			urls.Thumb = &t
		}
	}
	v.URLs = urls
	return v, nil
}

// ProcessAttachment is the media:process job. It returns nil when the attachment ended up ready
// or rejected (a rejection is an answer, not a failure) and an error only when a retry could
// help. It is safe to run twice for one attachment.
func (s *Service) ProcessAttachment(ctx context.Context, id uuid.UUID) error {
	u, err := s.uploadsOn()
	if err != nil {
		return err
	}
	if u.Media == nil {
		return errors.New("service: media processor is not configured")
	}
	a, err := s.st.ClaimAttachmentForProcessing(ctx, id)
	if store.IsNoRows(err) {
		return nil // not uploaded yet, already finished, or gone
	}
	if err != nil {
		return err
	}
	if a.StagingKey == nil {
		return s.reject(ctx, u, a, "File tidak ditemukan. Unggah ulang.")
	}

	work, err := os.MkdirTemp(u.TmpDir, "media-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	src := filepath.Join(work, "source")
	if err := download(ctx, u.Blobs, *a.StagingKey, src); errors.Is(err, storage.ErrNotFound) {
		return s.reject(ctx, u, a, "File tidak ditemukan. Unggah ulang.")
	} else if err != nil {
		return err
	}

	res, err := u.Media.Process(ctx, src, media.Kind(a.Kind), filepath.Join(work, "out"))
	var rej *media.RejectError
	if errors.As(err, &rej) {
		return s.reject(ctx, u, a, rej.Reason)
	}
	if err != nil {
		return err
	}

	prefix := a.PactID.String() + "/" + a.ID.String()
	mainKey := prefix + res.Main.Ext
	if err := putFile(ctx, u.Blobs, mainKey, res.Main); err != nil {
		return err
	}
	var thumbKey *string
	if res.Thumb != nil {
		k := prefix + "_thumb" + res.Thumb.Ext
		if err := putFile(ctx, u.Blobs, k, *res.Thumb); err != nil {
			return err
		}
		thumbKey = &k
	}
	if err := s.st.MarkAttachmentReady(ctx, store.MarkAttachmentReadyParams{
		ID: a.ID, MediaKey: &mainKey, ThumbKey: thumbKey, SniffedMime: &res.SniffedMIME, StoredBytes: &res.Main.Size,
		Width: nonZero32(res.Width), Height: nonZero32(res.Height), DurationMs: nonZero32(res.DurationMS), ReadyAt: ptr(s.clock.Now()),
	}); err != nil {
		return err
	}
	// The raw upload is no longer needed. A failure here only leaves an object the 24 h sweep
	// of staging would otherwise have removed with its row, so it is logged, not retried.
	if err := u.Blobs.Delete(ctx, storage.Staging, *a.StagingKey); err != nil {
		u.Log.Warn("delete staging object", "attachment", a.ID, "err", err)
	}
	return nil
}

func (s *Service) reject(ctx context.Context, u *UploadDeps, a store.Attachment, reason string) error {
	if err := s.st.RejectAttachment(ctx, store.RejectAttachmentParams{ID: a.ID, RejectReason: &reason}); err != nil {
		return err
	}
	if a.StagingKey != nil {
		if err := u.Blobs.Delete(ctx, storage.Staging, *a.StagingKey); err != nil {
			u.Log.Warn("delete rejected upload", "attachment", a.ID, "err", err)
		}
	}
	return nil
}

func download(ctx context.Context, b storage.BlobStore, key, dst string) error {
	rc, err := b.Get(ctx, storage.Staging, key)
	if err != nil {
		return err
	}
	defer rc.Close()
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, rc); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func putFile(ctx context.Context, b storage.BlobStore, key string, f media.File) error {
	file, err := os.Open(f.Path)
	if err != nil {
		return err
	}
	defer file.Close()
	return b.Put(ctx, storage.Media, key, file, f.Size, f.ContentType)
}

// UploadSweep reports what CollectUploads did.
type UploadSweep struct {
	Removed  int // attachments deleted with their objects
	Requeued int // completed uploads whose processing task was queued again
}

// CollectUploads is the uploads:gc job (ARCHITECTURE §6): it deletes slots never completed or
// never processed after 24 h, deletes processed attachments that never joined a proof after
// 7 days, and queues processing again for completed uploads that nothing picked up.
func (s *Service) CollectUploads(ctx context.Context) (UploadSweep, error) {
	u, err := s.uploadsOn()
	if err != nil {
		return UploadSweep{}, err
	}
	var out UploadSweep
	now := s.clock.Now()

	remove := func(list func(limit int32) ([]store.Attachment, error)) error {
		for range gcMaxBatches {
			rows, err := list(gcBatch)
			if err != nil {
				return err
			}
			for _, a := range rows {
				for _, k := range []struct {
					b   storage.Bucket
					key *string
				}{{storage.Staging, a.StagingKey}, {storage.Media, a.MediaKey}, {storage.Media, a.ThumbKey}} {
					if k.key != nil {
						if err := u.Blobs.Delete(ctx, k.b, *k.key); err != nil {
							return err
						}
					}
				}
				if err := s.st.DeleteAttachment(ctx, a.ID); err != nil {
					return err
				}
				out.Removed++
			}
			if len(rows) < gcBatch {
				return nil
			}
		}
		return nil
	}
	if err := remove(func(n int32) ([]store.Attachment, error) {
		return s.st.ListStaleUploads(ctx, store.ListStaleUploadsParams{CreatedAt: now.Add(-staleUploadAge), Limit: n})
	}); err != nil {
		return out, err
	}
	if err := remove(func(n int32) ([]store.Attachment, error) {
		return s.st.ListOrphanAttachments(ctx, store.ListOrphanAttachmentsParams{CreatedAt: now.Add(-orphanMediaAge), Limit: n})
	}); err != nil {
		return out, err
	}

	stuck, err := s.st.ListStuckUploaded(ctx, store.ListStuckUploadedParams{CreatedAt: now.Add(-requeueAfter), Limit: gcBatch})
	if err != nil {
		return out, err
	}
	for _, a := range stuck {
		if err := u.Queue.Enqueue(ctx, a.ID); err != nil {
			return out, err
		}
		out.Requeued++
	}
	return out, nil
}

func ptr[T any](v T) *T { return &v }

func nonZero32[T ~int](v T) *int32 {
	if v == 0 {
		return nil
	}
	n := int32(v)
	return &n
}
