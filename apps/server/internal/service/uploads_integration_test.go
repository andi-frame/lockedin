//go:build integration

package service

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/media"
	"github.com/andi-frame/lockedin/apps/server/internal/storage"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

type fakeMediaQueue struct {
	busy     bool
	enqueued []uuid.UUID
	err      error
}

func (q *fakeMediaQueue) Enqueue(_ context.Context, id uuid.UUID) error {
	if q.err != nil {
		return q.err
	}
	q.enqueued = append(q.enqueued, id)
	return nil
}
func (q *fakeMediaQueue) Busy(context.Context) (bool, error) { return q.busy, nil }

type uploadRig struct {
	*fixture
	blobs *storage.FS
	queue *fakeMediaQueue
	pact  store.Pact
}

func newUploadRig(t *testing.T) *uploadRig {
	t.Helper()
	f := newFixture(t)
	blobs, err := storage.NewFS(t.TempDir(), "test-secret-test-secret", "http://api.test")
	if err != nil {
		t.Fatal(err)
	}
	q := &fakeMediaQueue{}
	f.svc.WithUploads(UploadDeps{
		Blobs: blobs, Queue: q, TmpDir: t.TempDir(),
		Limits: UploadLimits{ImageMaxBytes: 15 << 20, VideoMaxBytes: 200 << 20, FileMaxBytes: 20 << 20, PactQuotaBytes: 1 << 30},
		Media:  media.New(mediaTools(), media.DefaultLimits()),
	})
	return &uploadRig{fixture: f, blobs: blobs, queue: q, pact: f.activePactWith(nil)}
}

func mediaTools() media.Tools {
	return media.Tools{FFmpeg: "ffmpeg", FFprobe: "ffprobe", Vips: "vips", VipsHeader: "vipsheader"}
}

func needMediaTools(t *testing.T) {
	t.Helper()
	for _, b := range []string{"ffmpeg", "ffprobe", "vips", "vipsheader"} {
		if _, err := exec.LookPath(b); err != nil {
			t.Skipf("%s is not on PATH; install ffmpeg and libvips", b)
		}
	}
}

func (r *uploadRig) intent(user uuid.UUID, kind, mime string, bytes int64) (UploadIntent, error) {
	return r.svc.CreateUpload(r.ctx, user, UploadInput{PactID: r.pact.ID, Kind: kind, Mime: mime, Bytes: bytes})
}

// upload runs the first two steps for real data: an intent, then the bytes into staging.
func (r *uploadRig) upload(user uuid.UUID, kind, mime string, data []byte) uuid.UUID {
	r.t.Helper()
	in, err := r.intent(user, kind, mime, int64(len(data)))
	r.must(err)
	att, err := r.st.GetAttachment(r.ctx, in.AttachmentID)
	r.must(err)
	r.must(r.blobs.Put(r.ctx, storage.Staging, *att.StagingKey, bytes.NewReader(data), int64(len(data)), mime))
	return in.AttachmentID
}

func (r *uploadRig) attachment(id uuid.UUID) store.Attachment {
	r.t.Helper()
	a, err := r.st.GetAttachment(r.ctx, id)
	r.must(err)
	return a
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), "x.png")
	if b, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=640x480", "-frames:v", "1", out).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v\n%s", err, b)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// ---------------------------------------------------------------- createUpload

func TestCreateUploadReturnsASignedSlot(t *testing.T) {
	r := newUploadRig(t)
	in, err := r.intent(r.doer.ID, "image", "image/webp", 123456)
	r.must(err)
	if in.PutURL == "" || in.Headers["Content-Type"] != "image/webp" || !in.ExpiresAt.After(time.Now()) {
		t.Fatalf("intent = %+v", in)
	}
	a := r.attachment(in.AttachmentID)
	if a.Status != "awaiting_upload" || a.OwnerID != r.doer.ID || a.PactID != r.pact.ID || a.Kind != "image" || a.StagingKey == nil || *a.DeclaredBytes != 123456 {
		t.Fatalf("row = %+v", a)
	}
	if !storage.ValidKey(*a.StagingKey) {
		t.Fatalf("staging key %q is not a valid storage key", *a.StagingKey)
	}
}

func TestCreateUploadIsForMembersOfAnActivePactOnly(t *testing.T) {
	r := newUploadRig(t)
	stranger := r.user("orang@tepati.test", "Orang")
	if _, err := r.intent(stranger.ID, "image", "image/webp", 1000); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-member err = %v, want ErrNotFound (invariant 8)", err)
	}
	if _, err := r.svc.CreateUpload(r.ctx, r.doer.ID, UploadInput{PactID: uuid.New(), Kind: "image", Mime: "image/webp", Bytes: 1000}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown pact err = %v", err)
	}
	// A draft pact has no check-ins to attach proof to.
	draft, err := r.svc.CreateDraft(r.ctx, r.backer.ID, DraftInput{Title: "Draft", Terms: workedTerms(r.backer.ID)})
	r.must(err)
	if _, err := r.svc.CreateUpload(r.ctx, r.backer.ID, UploadInput{PactID: draft.ID, Kind: "image", Mime: "image/webp", Bytes: 1000}); !errors.Is(err, ErrPactState) {
		t.Fatalf("draft pact err = %v, want ErrPactState", err)
	}
}

func TestCreateUploadEnforcesTypesAndLimits(t *testing.T) {
	r := newUploadRig(t)
	for _, tc := range []struct {
		name, kind, mime string
		bytes            int64
		want             error
	}{
		{"unknown kind", "audio", "audio/mpeg", 1000, ErrUploadUnsupported},
		{"mime of another kind", "image", "video/mp4", 1000, ErrUploadUnsupported},
		{"executable", "file", "application/x-msdownload", 1000, ErrUploadUnsupported},
		{"image too big", "image", "image/jpeg", 15<<20 + 1, ErrUploadTooLarge},
		{"video too big", "video", "video/mp4", 200<<20 + 1, ErrUploadTooLarge},
		{"pdf too big", "file", "application/pdf", 20<<20 + 1, ErrUploadTooLarge},
		{"empty", "image", "image/png", 0, ErrUploadUnsupported},
	} {
		if _, err := r.intent(r.doer.ID, tc.kind, tc.mime, tc.bytes); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
	for _, mime := range []string{"image/jpeg", "image/png", "image/webp", "image/heic", "image/heif", "image/avif", "image/gif"} {
		if _, err := r.intent(r.doer.ID, "image", mime, 1000); err != nil {
			t.Errorf("%s was refused: %v", mime, err)
		}
	}
	for _, mime := range []string{"video/mp4", "video/quicktime", "video/webm"} {
		if _, err := r.intent(r.doer.ID, "video", mime, 1000); err != nil {
			t.Errorf("%s was refused: %v", mime, err)
		}
	}
}

func TestCreateUploadStopsAtThePactQuota(t *testing.T) {
	r := newUploadRig(t)
	r.svc.up.Limits.PactQuotaBytes = 10_000
	if _, err := r.intent(r.doer.ID, "image", "image/png", 6000); err != nil {
		t.Fatal(err)
	}
	if _, err := r.intent(r.backer.ID, "image", "image/png", 6000); !errors.Is(err, ErrUploadQuota) {
		t.Fatalf("second upload err = %v, want ErrUploadQuota (the quota is per pact, not per user)", err)
	}
	if _, err := r.intent(r.backer.ID, "image", "image/png", 4000); err != nil {
		t.Fatalf("an upload that still fits was refused: %v", err)
	}
}

func TestCreateUploadAnswersBusyWhenTheMediaQueueIsFull(t *testing.T) {
	r := newUploadRig(t)
	r.queue.busy = true
	if _, err := r.intent(r.doer.ID, "image", "image/png", 1000); !errors.Is(err, ErrUploadQueueBusy) {
		t.Fatalf("err = %v", err)
	}
}

// ---------------------------------------------------------------- completeUpload

func TestCompleteUploadQueuesProcessingOnce(t *testing.T) {
	r := newUploadRig(t)
	id := r.upload(r.doer.ID, "image", "image/png", []byte("0123456789"))
	a, err := r.svc.CompleteUpload(r.ctx, r.doer.ID, id)
	r.must(err)
	if a.Status != "uploaded" || len(r.queue.enqueued) != 1 || r.queue.enqueued[0] != id {
		t.Fatalf("status %s, enqueued %v", a.Status, r.queue.enqueued)
	}
	// The browser may retry the call; the attachment is simply reported again.
	a, err = r.svc.CompleteUpload(r.ctx, r.doer.ID, id)
	r.must(err)
	if a.Status != "uploaded" {
		t.Fatalf("repeat status = %s", a.Status)
	}
}

func TestCompleteUploadRejectsASizeThatDiffersFromTheDeclaration(t *testing.T) {
	r := newUploadRig(t)
	in, err := r.intent(r.doer.ID, "image", "image/png", 100)
	r.must(err)
	att := r.attachment(in.AttachmentID)
	r.must(r.blobs.Put(r.ctx, storage.Staging, *att.StagingKey, strings.NewReader("only ten b"), 10, "image/png"))

	if _, err := r.svc.CompleteUpload(r.ctx, r.doer.ID, in.AttachmentID); !errors.Is(err, ErrUploadSizeMismatch) {
		t.Fatalf("err = %v", err)
	}
	got := r.attachment(in.AttachmentID)
	if got.Status != "rejected" || got.RejectReason == nil {
		t.Fatalf("row = %+v", got)
	}
	if _, err := r.blobs.Head(r.ctx, storage.Staging, *att.StagingKey); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("the mismatched object stayed in staging: %v", err)
	}
	if len(r.queue.enqueued) != 0 {
		t.Fatal("a rejected upload was queued for processing")
	}
}

func TestCompleteUploadWithoutTheBytesIsAMismatch(t *testing.T) {
	r := newUploadRig(t)
	in, err := r.intent(r.doer.ID, "image", "image/png", 100)
	r.must(err)
	if _, err := r.svc.CompleteUpload(r.ctx, r.doer.ID, in.AttachmentID); !errors.Is(err, ErrUploadSizeMismatch) {
		t.Fatalf("err = %v", err)
	}
	// Nothing was uploaded, so the slot stays usable: the client can still PUT and complete.
	if got := r.attachment(in.AttachmentID); got.Status != "awaiting_upload" {
		t.Fatalf("status = %s", got.Status)
	}
}

func TestCompleteUploadBelongsToTheUploader(t *testing.T) {
	r := newUploadRig(t)
	id := r.upload(r.doer.ID, "image", "image/png", []byte("0123456789"))
	if _, err := r.svc.CompleteUpload(r.ctx, r.backer.ID, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another member completing it: %v", err)
	}
	if _, err := r.svc.CompleteUpload(r.ctx, r.doer.ID, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id: %v", err)
	}
}

func TestCompleteUploadWhenTheQueueIsFullKeepsTheSlot(t *testing.T) {
	r := newUploadRig(t)
	id := r.upload(r.doer.ID, "image", "image/png", []byte("0123456789"))
	r.queue.busy = true
	if _, err := r.svc.CompleteUpload(r.ctx, r.doer.ID, id); !errors.Is(err, ErrUploadQueueBusy) {
		t.Fatalf("err = %v", err)
	}
	r.queue.busy = false
	if _, err := r.svc.CompleteUpload(r.ctx, r.doer.ID, id); err != nil {
		t.Fatalf("retry after the backlog cleared: %v", err)
	}
}

func TestACompleteWhoseEnqueueFailsIsRecoveredByTheSweep(t *testing.T) {
	r := newUploadRig(t)
	id := r.upload(r.doer.ID, "image", "image/png", []byte("0123456789"))
	r.queue.err = errors.New("redis down")
	a, err := r.svc.CompleteUpload(r.ctx, r.doer.ID, id)
	r.must(err) // the bytes are safe and the row says uploaded; queueing is retried by the sweep
	if a.Status != "uploaded" {
		t.Fatalf("status = %s", a.Status)
	}
	r.queue.err = nil
	r.clock.Advance(10 * time.Minute)
	if _, err := r.svc.CollectUploads(r.ctx); err != nil {
		t.Fatal(err)
	}
	if len(r.queue.enqueued) != 1 || r.queue.enqueued[0] != id {
		t.Fatalf("enqueued after sweep = %v", r.queue.enqueued)
	}
}

// ---------------------------------------------------------------- processing

func TestProcessAttachmentMakesAnImageReady(t *testing.T) {
	needMediaTools(t)
	r := newUploadRig(t)
	id := r.upload(r.doer.ID, "image", "image/png", pngBytes(t))
	_, err := r.svc.CompleteUpload(r.ctx, r.doer.ID, id)
	r.must(err)
	r.must(r.svc.ProcessAttachment(r.ctx, id))

	a := r.attachment(id)
	if a.Status != "ready" || a.MediaKey == nil || a.ThumbKey == nil || a.SniffedMime == nil || *a.SniffedMime != "image/png" || a.ReadyAt == nil {
		t.Fatalf("row = %+v", a)
	}
	if a.Width == nil || *a.Width != 640 || *a.Height != 480 || a.StoredBytes == nil || *a.StoredBytes <= 0 {
		t.Fatalf("metrics = %v x %v, %v bytes", a.Width, a.Height, a.StoredBytes)
	}
	info, err := r.blobs.Head(r.ctx, storage.Media, *a.MediaKey)
	if err != nil || info.ContentType != "image/webp" || info.Size != *a.StoredBytes {
		t.Fatalf("media object = %+v, %v", info, err)
	}
	if _, err := r.blobs.Head(r.ctx, storage.Media, *a.ThumbKey); err != nil {
		t.Fatalf("thumbnail missing: %v", err)
	}
	if _, err := r.blobs.Head(r.ctx, storage.Staging, *a.StagingKey); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("the raw upload was kept in staging: %v", err)
	}
	// A task delivered twice does no harm.
	r.must(r.svc.ProcessAttachment(r.ctx, id))
	if r.attachment(id).Status != "ready" {
		t.Fatal("a repeat run changed the attachment")
	}
}

func TestProcessAttachmentRejectsFilesThatAreNotWhatTheyClaim(t *testing.T) {
	needMediaTools(t)
	r := newUploadRig(t)
	// A PNG uploaded as a video.
	id := r.upload(r.doer.ID, "video", "video/mp4", pngBytes(t))
	_, err := r.svc.CompleteUpload(r.ctx, r.doer.ID, id)
	r.must(err)
	r.must(r.svc.ProcessAttachment(r.ctx, id)) // a rejection is a result, not a failure to retry

	a := r.attachment(id)
	if a.Status != "rejected" || a.RejectReason == nil || *a.RejectReason == "" {
		t.Fatalf("row = %+v", a)
	}
	if _, err := r.blobs.Head(r.ctx, storage.Staging, *a.StagingKey); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("a rejected file stayed in staging: %v", err)
	}
	if a.MediaKey != nil {
		t.Fatal("a rejected file got a media key")
	}
}

func TestProcessAttachmentRetriesWhenInterrupted(t *testing.T) {
	needMediaTools(t)
	r := newUploadRig(t)
	id := r.upload(r.doer.ID, "image", "image/png", pngBytes(t))
	_, err := r.svc.CompleteUpload(r.ctx, r.doer.ID, id)
	r.must(err)

	ctx, cancel := context.WithCancel(r.ctx)
	cancel()
	if err := r.svc.ProcessAttachment(ctx, id); err == nil {
		t.Fatal("a cancelled run reported success")
	}
	if got := r.attachment(id).Status; got == "rejected" || got == "ready" {
		t.Fatalf("an interrupted run left the attachment %s; it must stay retryable", got)
	}
	r.must(r.svc.ProcessAttachment(r.ctx, id)) // the retry finishes the job
	if r.attachment(id).Status != "ready" {
		t.Fatalf("status after retry = %s", r.attachment(id).Status)
	}
}

func TestProcessAttachmentIgnoresSlotsThatWereNeverCompleted(t *testing.T) {
	r := newUploadRig(t)
	in, err := r.intent(r.doer.ID, "image", "image/png", 100)
	r.must(err)
	r.must(r.svc.ProcessAttachment(r.ctx, in.AttachmentID))
	if got := r.attachment(in.AttachmentID).Status; got != "awaiting_upload" {
		t.Fatalf("status = %s: processing must only start after complete", got)
	}
	r.must(r.svc.ProcessAttachment(r.ctx, uuid.New())) // an unknown id is not an error either
}

// ---------------------------------------------------------------- reading

func TestGetAttachmentShowsSignedURLsToMembersOnly(t *testing.T) {
	needMediaTools(t)
	r := newUploadRig(t)
	id := r.upload(r.doer.ID, "image", "image/png", pngBytes(t))
	_, err := r.svc.CompleteUpload(r.ctx, r.doer.ID, id)
	r.must(err)

	v, err := r.svc.GetAttachment(r.ctx, r.doer.ID, id)
	r.must(err)
	if v.Status != "uploaded" || v.URLs != nil {
		t.Fatalf("before processing: %+v", v)
	}
	r.must(r.svc.ProcessAttachment(r.ctx, id))

	for _, member := range []uuid.UUID{r.doer.ID, r.backer.ID} { // the reviewer needs to see it too
		v, err = r.svc.GetAttachment(r.ctx, member, id)
		r.must(err)
		if v.URLs == nil || v.URLs.Original == "" || v.URLs.Thumb == nil || v.URLs.ExpiresAt.Before(time.Now().Add(4*time.Minute)) {
			t.Fatalf("urls for %s = %+v", member, v.URLs)
		}
	}
	stranger := r.user("orang@tepati.test", "Orang")
	if _, err := r.svc.GetAttachment(r.ctx, stranger.ID, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-member err = %v", err)
	}
	if _, err := r.svc.GetAttachment(r.ctx, r.doer.ID, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id err = %v", err)
	}
}

// ---------------------------------------------------------------- garbage collection

func TestCollectUploadsRemovesStaleSlotsAndOrphans(t *testing.T) {
	r := newUploadRig(t)

	abandoned := r.upload(r.doer.ID, "image", "image/png", []byte("0123456789")) // PUT done, never completed
	staleKey := *r.attachment(abandoned).StagingKey
	fresh := r.upload(r.doer.ID, "image", "image/png", []byte("0123456789"))

	r.clock.Advance(25 * time.Hour)
	freshAfter := r.upload(r.doer.ID, "image", "image/png", []byte("0123456789")) // created "now"

	res, err := r.svc.CollectUploads(r.ctx)
	r.must(err)
	if res.Removed != 2 {
		t.Fatalf("removed %d, want the two stale slots (%+v)", res.Removed, res)
	}
	for _, id := range []uuid.UUID{abandoned, fresh} {
		if _, err := r.st.GetAttachment(r.ctx, id); !store.IsNoRows(err) {
			t.Fatalf("stale attachment %s survived: %v", id, err)
		}
	}
	if _, err := r.blobs.Head(r.ctx, storage.Staging, staleKey); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("the stale staging object survived: %v", err)
	}
	if r.attachment(freshAfter).Status != "awaiting_upload" {
		t.Fatal("a slot younger than 24 h was collected")
	}
}

func TestCollectUploadsKeepsAttachmentsThatBelongToAProof(t *testing.T) {
	r := newUploadRig(t)
	// A "ready" attachment: one orphan, one attached to a proof.
	orphan, attached := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{orphan, attached} {
		key := r.pact.ID.String() + "/" + id.String() + ".webp"
		r.must(r.blobs.Put(r.ctx, storage.Media, key, strings.NewReader("x"), 1, "image/webp"))
		_, err := r.st.Pool.Exec(r.ctx, `insert into attachments (id, owner_id, pact_id, kind, status, media_key, stored_bytes, declared_bytes, created_at, ready_at)
			values ($1, $2, $3, 'image', 'ready', $4, 1, 1, $5, $5)`, id, r.doer.ID, r.pact.ID, key, r.clock.Now())
		r.must(err)
	}
	ci := r.checkIn(r.pact.ID, r.doer.ID, 2)
	proofRow, err := r.st.InsertProof(r.ctx, store.InsertProofParams{ID: uuid.New(), CheckInID: ci.ID, BodyDoc: []byte(`{"type":"doc"}`), BodyText: "x", Links: []byte(`[]`)})
	r.must(err)
	_, err = r.st.AttachToProof(r.ctx, store.AttachToProofParams{ProofID: &proofRow.ID, Ids: []uuid.UUID{attached}, OwnerID: r.doer.ID, PactID: r.pact.ID})
	r.must(err)

	r.clock.Advance(8 * 24 * time.Hour)
	res, err := r.svc.CollectUploads(r.ctx)
	r.must(err)
	if res.Removed != 1 {
		t.Fatalf("removed %d, want only the orphan (%+v)", res.Removed, res)
	}
	if _, err := r.st.GetAttachment(r.ctx, orphan); !store.IsNoRows(err) {
		t.Fatalf("orphan survived: %v", err)
	}
	if _, err := r.st.GetAttachment(r.ctx, attached); err != nil {
		t.Fatalf("an attachment of a proof was collected: %v", err)
	}
	if _, err := r.blobs.Head(r.ctx, storage.Media, r.pact.ID.String()+"/"+orphan.String()+".webp"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("the orphan's media object survived: %v", err)
	}
}
