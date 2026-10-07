package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newProcessor(t *testing.T) *Processor {
	t.Helper()
	return New(testTools(t), DefaultLimits())
}

func dims(t *testing.T, path string) (w, h int) {
	t.Helper()
	read := func(field string) int {
		out, err := exec.Command("vipsheader", "-f", field, path).Output()
		if err != nil {
			t.Fatalf("vipsheader %s %s: %v", field, path, err)
		}
		n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
		return n
	}
	return read("width"), read("height")
}

type probe struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
	Format struct {
		Duration string            `json:"duration"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
}

func ffprobe(t *testing.T, path string) probe {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_format", "-show_streams", path).Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v", path, err)
	}
	var p probe
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// riffChunks lists the chunk names of a WebP file, to prove no EXIF or XMP chunk survived.
func riffChunks(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil || len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		t.Fatalf("%s is not a WebP file (%v)", path, err)
	}
	var names []string
	for i := 12; i+8 <= len(b); {
		size := int(b[i+4]) | int(b[i+5])<<8 | int(b[i+6])<<16 | int(b[i+7])<<24
		names = append(names, string(b[i:i+4]))
		i += 8 + size + size%2
	}
	return names
}

func TestSniffByMagicBytesNotByName(t *testing.T) {
	tools := testTools(t)
	dir := t.TempDir()
	_ = tools
	png := pngFile(t, filepath.Join(dir, "x.png"))
	asMP4 := filepath.Join(dir, "looks-like.mp4")
	if err := os.Rename(png, asMP4); err != nil {
		t.Fatal(err)
	}
	mp4 := clip(t, filepath.Join(dir, "c.mp4"), "320x240", 5, 1, false)
	jpg := photo12MPWithGPS(t, dir)
	pdf := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	junk := filepath.Join(dir, "junk.jpg")
	_ = os.WriteFile(junk, []byte("MZ this is an executable"), 0o600)
	html := filepath.Join(dir, "page.png")
	_ = os.WriteFile(html, []byte("<html><script>alert(1)</script></html>"), 0o600)

	for _, tc := range []struct {
		path, mime string
		kind       Kind
	}{
		{asMP4, "image/png", KindImage},
		{mp4, "video/mp4", KindVideo},
		{jpg, "image/jpeg", KindImage},
		{pdf, "application/pdf", KindFile},
	} {
		mime, kind, err := Sniff(tc.path)
		if err != nil || mime != tc.mime || kind != tc.kind {
			t.Errorf("Sniff(%s) = %q %q %v, want %q %q", filepath.Base(tc.path), mime, kind, err, tc.mime, tc.kind)
		}
	}
	for _, p := range []string{junk, html} {
		if _, _, err := Sniff(p); !errors.Is(err, ErrUnsupported) {
			t.Errorf("Sniff(%s) = %v, want ErrUnsupported", filepath.Base(p), err)
		}
	}
}

// PLAN 4.2 golden test 1: a 12 MP JPEG with GPS becomes a WebP of at most 2048 px with no EXIF.
func TestPhotoWithGPSBecomesAStrippedWebP(t *testing.T) {
	p := newProcessor(t)
	dir := t.TempDir()
	src := photo12MPWithGPS(t, dir)

	// The fixture really carries GPS (otherwise "no EXIF in the output" proves nothing).
	hdr, err := exec.Command("vipsheader", "-a", src).Output()
	if err != nil || !bytes.Contains(bytes.ToLower(hdr), []byte("gpslatitude")) {
		t.Fatalf("the fixture has no GPS tags (%v):\n%s", err, hdr)
	}

	res, err := p.Process(context.Background(), src, KindImage, filepath.Join(dir, "work"))
	if err != nil {
		t.Fatal(err)
	}
	if res.SniffedMIME != "image/jpeg" || res.Main.ContentType != "image/webp" || res.Main.Ext != ".webp" {
		t.Fatalf("result = %+v", res)
	}
	w, h := dims(t, res.Main.Path)
	if w != 2048 || h != 1536 || res.Width != 2048 || res.Height != 1536 {
		t.Fatalf("output is %dx%d (result says %dx%d), want 2048x1536", w, h, res.Width, res.Height)
	}
	for _, name := range riffChunks(t, res.Main.Path) {
		if name == "EXIF" || name == "XMP " {
			t.Fatalf("the output kept a %q chunk", name)
		}
	}
	raw, _ := os.ReadFile(res.Main.Path)
	if bytes.Contains(raw, []byte("Exif")) {
		t.Fatal("the output still contains EXIF bytes")
	}
	if res.Thumb == nil {
		t.Fatal("no thumbnail")
	}
	if tw, th := dims(t, res.Thumb.Path); max(tw, th) != 480 {
		t.Fatalf("thumbnail is %dx%d, want a longest side of 480", tw, th)
	}
	if res.Main.Size <= 0 || res.Main.Size > 15<<20 {
		t.Fatalf("size = %d", res.Main.Size)
	}
}

func TestSmallImagesAreNotUpscaled(t *testing.T) {
	p := newProcessor(t)
	dir := t.TempDir()
	src := pngFile(t, filepath.Join(dir, "small.png"))
	res, err := p.Process(context.Background(), src, KindImage, filepath.Join(dir, "work"))
	if err != nil {
		t.Fatal(err)
	}
	if w, h := dims(t, res.Main.Path); w != 320 || h != 240 {
		t.Fatalf("a 320x240 image became %dx%d", w, h)
	}
}

// PLAN 4.2 golden test 2: a PNG renamed .mp4 is rejected.
func TestPNGRenamedToMP4IsRejected(t *testing.T) {
	p := newProcessor(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "holiday.mp4")
	pngFile(t, src)
	_, err := p.Process(context.Background(), src, KindVideo, filepath.Join(dir, "work"))
	var rej *RejectError
	if !errors.As(err, &rej) || rej.Reason == "" {
		t.Fatalf("err = %v, want a RejectError with a reason for the uploader", err)
	}
}

func TestContentThatIsNotMediaIsRejected(t *testing.T) {
	p := newProcessor(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "evil.png")
	_ = os.WriteFile(src, []byte("<svg xmlns='http://www.w3.org/2000/svg' onload='alert(1)'/>"), 0o600)
	var rej *RejectError
	if _, err := p.Process(context.Background(), src, KindImage, filepath.Join(dir, "work")); !errors.As(err, &rej) {
		t.Fatalf("err = %v", err)
	}
}

// PLAN 4.2 golden test 3: a 200-second video is rejected.
func TestVideoOverTheDurationLimitIsRejected(t *testing.T) {
	p := newProcessor(t)
	dir := t.TempDir()
	src := clip(t, filepath.Join(dir, "long.mp4"), "160x120", 1, 200, false)
	_, err := p.Process(context.Background(), src, KindVideo, filepath.Join(dir, "work"))
	var rej *RejectError
	if !errors.As(err, &rej) || !strings.Contains(rej.Reason, "180") {
		t.Fatalf("err = %v, want a rejection that names the 180 s limit", err)
	}
}

// PLAN 4.2 golden test 4: a 30-second 1080p video becomes 720p.
func TestFullHDVideoBecomes720p(t *testing.T) {
	p := newProcessor(t)
	dir := t.TempDir()
	src := clip(t, filepath.Join(dir, "lecture.mp4"), "1920x1080", 10, 30, true)
	res, err := p.Process(context.Background(), src, KindVideo, filepath.Join(dir, "work"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Main.ContentType != "video/mp4" || res.Main.Ext != ".mp4" {
		t.Fatalf("result = %+v", res)
	}
	info := ffprobe(t, res.Main.Path)
	var video, audio bool
	for _, s := range info.Streams {
		switch s.CodecType {
		case "video":
			video = true
			if s.CodecName != "h264" || s.Height != 720 || s.Width != 1280 {
				t.Errorf("video stream = %+v, want h264 1280x720", s)
			}
		case "audio":
			audio = s.CodecName == "aac"
		}
	}
	if !video || !audio {
		t.Errorf("streams = %+v (video %v, aac audio %v)", info.Streams, video, audio)
	}
	if d, _ := strconv.ParseFloat(info.Format.Duration, 64); d < 29 || d > 31 {
		t.Errorf("duration = %v s", info.Format.Duration)
	}
	if res.Width != 1280 || res.Height != 720 || res.DurationMS < 29000 || res.DurationMS > 31000 {
		t.Errorf("result metrics = %dx%d, %d ms", res.Width, res.Height, res.DurationMS)
	}
	if res.Thumb == nil || res.Thumb.ContentType != "image/jpeg" {
		t.Fatalf("poster = %+v", res.Thumb)
	}
	if len(info.Format.Tags) > 0 {
		for k := range info.Format.Tags {
			if k != "major_brand" && k != "minor_version" && k != "compatible_brands" && k != "encoder" {
				t.Errorf("container kept metadata tag %q", k)
			}
		}
	}
}

func TestSmallVideoIsNotUpscaled(t *testing.T) {
	p := newProcessor(t)
	dir := t.TempDir()
	src := clip(t, filepath.Join(dir, "small.mp4"), "320x240", 10, 2, false)
	res, err := p.Process(context.Background(), src, KindVideo, filepath.Join(dir, "work"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range ffprobe(t, res.Main.Path).Streams {
		if s.CodecType == "video" && (s.Width != 320 || s.Height != 240) {
			t.Fatalf("a 320x240 clip became %dx%d", s.Width, s.Height)
		}
	}
}

func TestDeclaredKindMustMatchTheContent(t *testing.T) {
	p := newProcessor(t)
	dir := t.TempDir()
	src := clip(t, filepath.Join(dir, "c.mp4"), "320x240", 5, 1, false)
	var rej *RejectError
	if _, err := p.Process(context.Background(), src, KindImage, filepath.Join(dir, "work")); !errors.As(err, &rej) {
		t.Fatalf("a video uploaded as an image: err = %v", err)
	}
}

func TestPDFIsKeptAsIs(t *testing.T) {
	p := newProcessor(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "catatan.pdf")
	body := []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n")
	_ = os.WriteFile(src, body, 0o600)
	res, err := p.Process(context.Background(), src, KindFile, filepath.Join(dir, "work"))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(res.Main.Path)
	if !bytes.Equal(got, body) || res.Main.ContentType != "application/pdf" || res.Main.Ext != ".pdf" || res.Thumb != nil {
		t.Fatalf("result = %+v", res)
	}
}

func TestOversizeInputsAreRejectedBeforeAnyTool(t *testing.T) {
	limits := DefaultLimits()
	limits.ImageMaxBytes = 1000
	p := New(testTools(t), limits)
	dir := t.TempDir()
	src := photo12MPWithGPS(t, dir)
	var rej *RejectError
	if _, err := p.Process(context.Background(), src, KindImage, filepath.Join(dir, "work")); !errors.As(err, &rej) {
		t.Fatalf("err = %v", err)
	}
}

func TestImageOverTheMegapixelLimitIsRejected(t *testing.T) {
	limits := DefaultLimits()
	limits.ImageMaxPixels = 10_000_000 // the 12 MP photo is over it
	p := New(testTools(t), limits)
	dir := t.TempDir()
	src := photo12MPWithGPS(t, dir)
	var rej *RejectError
	if _, err := p.Process(context.Background(), src, KindImage, filepath.Join(dir, "work")); !errors.As(err, &rej) || !strings.Contains(rej.Reason, "MP") {
		t.Fatalf("err = %v", err)
	}
}

func TestProcessingStopsWhenTheContextEnds(t *testing.T) {
	p := newProcessor(t)
	dir := t.TempDir()
	src := clip(t, filepath.Join(dir, "lecture.mp4"), "1920x1080", 10, 30, false)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := p.Process(ctx, src, KindVideo, filepath.Join(dir, "work"))
	var rej *RejectError
	if err == nil || errors.As(err, &rej) {
		t.Fatalf("err = %v, want a plain error (not a rejection) so the job can retry", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Fatalf("took %v after the deadline", time.Since(start))
	}
}
