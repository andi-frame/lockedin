//go:build integration

package http

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
	"github.com/andi-frame/lockedin/apps/server/internal/media"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/storage"
)

type uploadWiring struct {
	deps  service.UploadDeps
	blobs *storage.FS
	queue *fakeQueue
}

type fakeQueue struct {
	busy     bool
	enqueued []uuid.UUID
}

func (q *fakeQueue) Enqueue(_ context.Context, id uuid.UUID) error {
	q.enqueued = append(q.enqueued, id)
	return nil
}
func (q *fakeQueue) Busy(context.Context) (bool, error) { return q.busy, nil }

func newUploadWiring(t *testing.T) *uploadWiring {
	t.Helper()
	blobs, err := storage.NewFS(t.TempDir(), "test-secret-test-secret", "http://api.test")
	if err != nil {
		t.Fatal(err)
	}
	q := &fakeQueue{}
	return &uploadWiring{blobs: blobs, queue: q, deps: service.UploadDeps{
		Blobs: blobs, Queue: q, TmpDir: t.TempDir(),
		Limits: service.UploadLimits{ImageMaxBytes: 15 << 20, VideoMaxBytes: 200 << 20, FileMaxBytes: 20 << 20, PactQuotaBytes: 1 << 30},
		Media:  media.New(media.Tools{FFmpeg: "ffmpeg", FFprobe: "ffprobe", Vips: "vips", VipsHeader: "vipsheader"}, media.DefaultLimits()),
	}}
}

// signed sends a request to a presigned URL the way the browser does: no cookies, no CSRF.
func (s *stack) signed(method, rawURL string, body []byte, hdr map[string]string) (int, []byte) {
	s.t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		s.t.Fatal(err)
	}
	req := httptest.NewRequest(method, u.RequestURI(), bytes.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := s.app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	if err != nil {
		s.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe", "vips", "vipsheader"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s is not on PATH; install ffmpeg and libvips", bin)
		}
	}
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

func TestUploadFlowOverHTTP(t *testing.T) {
	png := testPNG(t)
	w := newUploadWiring(t)
	s := buildStack(t, w)
	sc := s.activePact()

	// 1. the intent
	var intent api.UploadIntent
	sc.bima.ok(201, "POST", "/api/v1/uploads", map[string]any{"pact_id": sc.pact.String(), "kind": "image", "mime": "image/png", "bytes": len(png)}).into(t, &intent)
	if intent.PutUrl == "" || intent.Headers["Content-Type"] != "image/png" {
		t.Fatalf("intent = %+v", intent)
	}

	// 2. the PUT goes to storage without a session; a body of the wrong length is refused
	if status, _ := s.signed("PUT", intent.PutUrl, append(append([]byte{}, png...), 'x'), intent.Headers); status < 400 {
		t.Fatalf("a PUT of the wrong length was accepted (%d)", status)
	}
	// completing before the bytes arrived is a size mismatch
	sc.bima.fails(400, api.UploadSizeMismatch, "POST", "/api/v1/uploads/"+intent.AttachmentId.String()+"/complete", nil)
	if status, msg := s.signed("PUT", intent.PutUrl, png, intent.Headers); status != 200 {
		t.Fatalf("PUT = %d %s", status, msg)
	}

	// 3. complete, then the worker's job, then read
	var att api.Attachment
	sc.bima.ok(200, "POST", "/api/v1/uploads/"+intent.AttachmentId.String()+"/complete", nil).into(t, &att)
	if att.Status != api.AttachmentStatusUploaded || len(w.queue.enqueued) != 1 {
		t.Fatalf("after complete: %s, %d queued", att.Status, len(w.queue.enqueued))
	}
	if err := s.svc.ProcessAttachment(context.Background(), intent.AttachmentId); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/attachments/" + intent.AttachmentId.String()
	for _, c := range []*client{sc.bima, sc.andi} { // the reviewer sees it too
		var got api.Attachment
		c.ok(200, "GET", path, nil).into(t, &got)
		if got.Status != api.AttachmentStatusReady || got.Urls == nil || got.Urls.Original == nil || got.Urls.Thumb == nil || got.Width == nil || *got.Width != 640 {
			t.Fatalf("%s sees %+v", c.name, got)
		}
		status, body := s.signed("GET", *got.Urls.Original, nil, nil)
		if status != 200 || len(body) < 16 || !bytes.HasPrefix(body, []byte("RIFF")) || !bytes.Equal(body[8:12], []byte("WEBP")) {
			t.Fatalf("signed GET = %d, %d bytes", status, len(body))
		}
	}
	stranger := s.register("Orang")
	stranger.fails(404, api.NotFound, "GET", path, nil)
	stranger.fails(404, api.NotFound, "POST", "/api/v1/uploads/"+intent.AttachmentId.String()+"/complete", nil)
}

func TestUploadIntentErrorsOverHTTP(t *testing.T) {
	w := newUploadWiring(t)
	s := buildStack(t, w)
	sc := s.activePact()
	intent := func(kind, mime string, bytes int64) map[string]any {
		return map[string]any{"pact_id": sc.pact.String(), "kind": kind, "mime": mime, "bytes": bytes}
	}

	sc.bima.fails(415, api.UploadUnsupportedType, "POST", "/api/v1/uploads", intent("image", "application/x-msdownload", 1000))
	sc.bima.fails(413, api.UploadTooLarge, "POST", "/api/v1/uploads", intent("image", "image/jpeg", 16<<20))
	sc.bima.fails(400, api.ValidationFailed, "POST", "/api/v1/uploads", intent("image", "image/jpeg", 0))
	sc.bima.fails(404, api.NotFound, "POST", "/api/v1/uploads", map[string]any{"pact_id": uuid.NewString(), "kind": "image", "mime": "image/png", "bytes": 10})
	s.register("Orang").fails(404, api.NotFound, "POST", "/api/v1/uploads", intent("image", "image/png", 10))
	s.anon().fails(401, api.AuthUnauthenticated, "POST", "/api/v1/uploads", intent("image", "image/png", 10))
	s.anon().fails(401, api.AuthUnauthenticated, "GET", "/api/v1/attachments/"+uuid.NewString(), nil)

	// A full media queue answers 503 with Retry-After, and the slot is not created.
	w.queue.busy = true
	r := sc.bima.call("POST", "/api/v1/uploads", intent("image", "image/png", 10))
	if r.status != 503 || r.problem(t).Code != api.UploadQueueBusy || r.hdr("Retry-After") == "" {
		t.Fatalf("busy answer = %d %s retry-after %q", r.status, r.body, r.hdr("Retry-After"))
	}
	w.queue.busy = false

	// The pact quota is per pact, not per user.
	w.deps.Limits.PactQuotaBytes = 1000
	s.svc.WithUploads(w.deps)
	sc.bima.ok(201, "POST", "/api/v1/uploads", intent("image", "image/png", 600))
	sc.andi.fails(409, api.UploadQuotaExceeded, "POST", "/api/v1/uploads", intent("image", "image/png", 600))
}
