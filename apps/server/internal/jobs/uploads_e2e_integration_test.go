//go:build integration

package jobs

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/media"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/storage"
	"github.com/andi-frame/lockedin/apps/server/internal/testdb"
)

// PLAN 4.2 end to end, with nothing faked: Garage, Redis, asynq, Postgres and the real tools.
// A member asks for a slot, PUTs a photo straight to Garage, completes it, and the worker's
// media server turns it into a ready WebP that a signed URL serves.
func TestUploadedPhotoBecomesReadyThroughGarageAndTheWorker(t *testing.T) {
	for _, bin := range []string{"ffmpeg", "ffprobe", "vips", "vipsheader"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s is not on PATH; install ffmpeg and libvips", bin)
		}
	}
	if testdb.Setting("S3_ACCESS_KEY") == "" {
		t.Skip("no S3_ACCESS_KEY; run `bun run garage:init`")
	}
	blobs, err := storage.NewS3(storage.S3Config{
		Endpoint: orDefault(testdb.Setting("S3_ENDPOINT"), "http://localhost:3900"), PublicEndpoint: testdb.Setting("S3_PUBLIC_ENDPOINT"),
		Region: testdb.Setting("S3_REGION"), AccessKey: testdb.Setting("S3_ACCESS_KEY"), SecretKey: testdb.Setting("S3_SECRET_KEY"),
		StagingBucket: orDefault(testdb.Setting("S3_BUCKET_STAGING"), "tepati-staging"), MediaBucket: orDefault(testdb.Setting("S3_BUCKET_MEDIA"), "tepati-media"),
	})
	if err != nil {
		t.Fatal(err)
	}

	st := testdb.New(t)
	rdb := testdb.RedisIn(t, testRedisDB)
	opt := redisOpt(rdb)
	clock := domain.NewFakeClock(at(2, 0, 0))
	svc := service.New(st, clock)
	pact, _, doer := activePact(t, st, clock, svc)

	queue := NewMediaQueue(opt, 100)
	defer queue.Close()
	svc.WithUploads(service.UploadDeps{
		Blobs: blobs, Queue: queue, TmpDir: t.TempDir(),
		Limits: service.UploadLimits{ImageMaxBytes: 15 << 20, VideoMaxBytes: 200 << 20, FileMaxBytes: 20 << 20, PactQuotaBytes: 1 << 30},
		Media:  media.New(media.Tools{FFmpeg: "ffmpeg", FFprobe: "ffprobe", Vips: "vips", VipsHeader: "vipsheader"}, media.DefaultLimits()),
	})

	startWorker(t, Options{
		Redis: opt, Svc: svc, Log: quietLog(), Concurrency: 2, Uploads: svc,
		Schedule: []Periodic{{"@every 1s", TypeOutboxRelay, QueueDefault, 10 * time.Second}},
	})

	png := filepath.Join(t.TempDir(), "photo.png")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=800x600", "-frames:v", "1", png).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v\n%s", err, out)
	}
	data, err := os.ReadFile(png)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	in, err := svc.CreateUpload(ctx, doer.ID, service.UploadInput{PactID: pact.ID, Kind: "image", Mime: "image/png", Bytes: int64(len(data))})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPut, in.PutURL, bytes.NewReader(data))
	for k, v := range in.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("PUT to Garage = %d", resp.StatusCode)
	}
	if _, err := svc.CompleteUpload(ctx, doer.ID, in.AttachmentID); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the attachment to become ready", 60*time.Second, func() bool {
		v, err := svc.GetAttachment(ctx, doer.ID, in.AttachmentID)
		if err != nil {
			t.Fatal(err)
		}
		if v.Status == "rejected" {
			t.Fatalf("rejected: %v", v.RejectReason)
		}
		return v.Status == "ready"
	})

	v, err := svc.GetAttachment(ctx, doer.ID, in.AttachmentID)
	if err != nil {
		t.Fatal(err)
	}
	if v.URLs == nil || v.Width == nil || *v.Width != 800 {
		t.Fatalf("view = %+v", v)
	}
	got, err := http.Get(v.URLs.Original)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(got.Body)
	_ = got.Body.Close()
	if got.StatusCode != 200 || len(body) < 12 || string(body[:4]) != "RIFF" || string(body[8:12]) != "WEBP" {
		t.Fatalf("signed GET = %d, %d bytes", got.StatusCode, len(body))
	}
	att, err := st.GetAttachment(ctx, in.AttachmentID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blobs.Head(ctx, storage.Staging, *att.StagingKey); err != storage.ErrNotFound {
		t.Fatalf("staging object after processing: %v", err)
	}
	t.Cleanup(func() {
		_ = blobs.Delete(ctx, storage.Media, *att.MediaKey)
		_ = blobs.Delete(ctx, storage.Media, *att.ThumbKey)
	})
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
