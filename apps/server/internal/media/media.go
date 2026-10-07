// Package media turns an uploaded file into what we store (SPEC §8, ARCHITECTURE §6): it sniffs
// the real type, enforces the limits, strips metadata and compresses images (libvips) and
// videos (ffmpeg). It works on local files only; moving them to and from object storage is the
// caller's job. The tools run through exec.CommandContext with argument slices, never a shell.
package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Kind is the attachment kind, as in the attachments table.
type Kind string

const (
	KindImage Kind = "image"
	KindVideo Kind = "video"
	KindFile  Kind = "file"
)

var ErrUnsupported = errors.New("media: unsupported file type")

// RejectError means the file itself is unacceptable, so retrying cannot help. Reason is written
// for the uploader and stored in attachments.reject_reason. Any other error from Process is
// environmental (a missing tool, a cancelled context) and worth a retry.
type RejectError struct{ Reason string }

func (e *RejectError) Error() string { return "media: rejected: " + e.Reason }

func reject(format string, a ...any) error { return &RejectError{Reason: fmt.Sprintf(format, a...)} }

// Tools are the paths of the external programs (FFMPEG_PATH and friends).
type Tools struct {
	FFmpeg, FFprobe, Vips, VipsHeader string
}

// Limits are SPEC §8's, with the pixel and output caps the SPEC states in prose.
type Limits struct {
	ImageMaxBytes       int64
	VideoMaxBytes       int64
	FileMaxBytes        int64
	VideoMaxSeconds     int
	ImageMaxPixels      int64
	VideoOutputMaxBytes int64 // a video still bigger than this after transcoding is rejected
}

func DefaultLimits() Limits {
	return Limits{
		ImageMaxBytes: 15 << 20, VideoMaxBytes: 200 << 20, FileMaxBytes: 20 << 20,
		VideoMaxSeconds: 180, ImageMaxPixels: 40_000_000, VideoOutputMaxBytes: 50 << 20,
	}
}

// File is one produced output.
type File struct {
	Path        string
	ContentType string
	Ext         string
	Size        int64
}

// Result is what Process produced. Main is the file for the media bucket and Thumb is the
// 480 px thumbnail (images) or the poster frame (videos); files have none.
type Result struct {
	Kind        Kind
	SniffedMIME string
	Main        File
	Thumb       *File
	Width       int
	Height      int
	DurationMS  int
}

const (
	imageLongSide = 2048
	thumbLongSide = 480
	videoMaxH     = 720
	vipsTimeout   = 90 * time.Second
	probeTimeout  = 30 * time.Second
	ffmpegTimeout = 15 * time.Minute
)

type Processor struct {
	tools  Tools
	limits Limits
}

func New(t Tools, l Limits) *Processor { return &Processor{tools: t, limits: l} }

// Process sniffs src, checks it against declared and the limits, and writes the outputs under
// workDir. For a file (pdf) Main points at src itself, so keep src until Main is uploaded.
func (p *Processor) Process(ctx context.Context, src string, declared Kind, workDir string) (Result, error) {
	mime, kind, err := Sniff(src)
	if errors.Is(err, ErrUnsupported) {
		return Result{}, reject("Jenis file tidak didukung. Pakai foto, video MP4/MOV/WebM, atau PDF.")
	}
	if err != nil {
		return Result{}, err
	}
	if kind != declared {
		return Result{}, reject("Isi file tidak cocok dengan jenis yang dipilih.")
	}
	st, err := os.Stat(src)
	if err != nil {
		return Result{}, err
	}
	if max := p.maxBytes(kind); st.Size() > max {
		return Result{}, reject("File terlalu besar. Batasnya %d MB.", max>>20)
	}
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return Result{}, err
	}
	res := Result{Kind: kind, SniffedMIME: mime}
	switch kind {
	case KindImage:
		err = p.image(ctx, src, workDir, &res)
	case KindVideo:
		err = p.video(ctx, src, workDir, &res)
	default:
		res.Main = File{Path: src, ContentType: mime, Ext: ".pdf", Size: st.Size()}
	}
	return res, err
}

func (p *Processor) maxBytes(k Kind) int64 {
	switch k {
	case KindImage:
		return p.limits.ImageMaxBytes
	case KindVideo:
		return p.limits.VideoMaxBytes
	}
	return p.limits.FileMaxBytes
}

func (p *Processor) image(ctx context.Context, src, dir string, res *Result) error {
	w, h, err := p.vipsDims(ctx, src)
	if err != nil {
		return err
	}
	if int64(w)*int64(h) > p.limits.ImageMaxPixels {
		return reject("Gambar terlalu besar (%d MP). Batasnya %d MP.", int64(w)*int64(h)/1_000_000, p.limits.ImageMaxPixels/1_000_000)
	}
	// "strip" drops EXIF, XMP and the GPS block; "--size down" never upscales; vips thumbnail
	// applies the EXIF orientation before stripping it, so photos stay upright.
	main := filepath.Join(dir, "main.webp")
	if err := p.vips(ctx, src, main+"[Q=80,strip]", imageLongSide); err != nil {
		return err
	}
	thumb := filepath.Join(dir, "thumb.webp")
	if err := p.vips(ctx, src, thumb+"[Q=75,strip]", thumbLongSide); err != nil {
		return err
	}
	ow, oh, err := p.vipsDims(ctx, main)
	if err != nil {
		return err
	}
	res.Width, res.Height = ow, oh
	if res.Main, err = describe(main, "image/webp", ".webp"); err != nil {
		return err
	}
	t, err := describe(thumb, "image/webp", ".webp")
	if err != nil {
		return err
	}
	res.Thumb = &t
	return nil
}

func (p *Processor) video(ctx context.Context, src, dir string, res *Result) error {
	in, err := p.probe(ctx, src)
	if err != nil {
		return err
	}
	if !in.hasVideo {
		return reject("File ini tidak berisi gambar video.")
	}
	if in.seconds > float64(p.limits.VideoMaxSeconds) {
		return reject("Video lebih panjang dari %d detik.", p.limits.VideoMaxSeconds)
	}
	out := filepath.Join(dir, "main.mp4")
	// -map_metadata -1 removes the container tags (location included); the audio map is optional
	// so a silent clip works; scale only shrinks, and -2 keeps the width even for H.264.
	err = p.ffmpeg(ctx, "-protocol_whitelist", "file", "-i", src,
		"-map", "0:v:0", "-map", "0:a:0?", "-map_metadata", "-1", "-map_chapters", "-1",
		"-vf", fmt.Sprintf(`scale=-2:min(%d\,ih)`, videoMaxH),
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "28", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "96k", "-movflags", "+faststart", "-threads", "2",
		// A container with no readable duration would slip past the check above, so cap the
		// output at one second over the limit and test the real length afterwards.
		"-t", strconv.Itoa(p.limits.VideoMaxSeconds+1), out)
	if err != nil {
		return err
	}
	if res.Main, err = describe(out, "video/mp4", ".mp4"); err != nil {
		return err
	}
	if res.Main.Size > p.limits.VideoOutputMaxBytes {
		return reject("Video masih terlalu besar setelah dikompres (%d MB). Potong videonya.", res.Main.Size>>20)
	}
	done, err := p.probe(ctx, out)
	if err != nil {
		return err
	}
	if done.seconds > float64(p.limits.VideoMaxSeconds) {
		return reject("Video lebih panjang dari %d detik.", p.limits.VideoMaxSeconds)
	}
	res.Width, res.Height, res.DurationMS = done.width, done.height, int(done.seconds*1000)

	poster := filepath.Join(dir, "poster.jpg")
	at := strconv.FormatFloat(min(1, done.seconds/2), 'f', 2, 64)
	if err := p.ffmpeg(ctx, "-ss", at, "-i", out, "-frames:v", "1", "-vf", fmt.Sprintf(`scale=-2:min(%d\,ih)`, thumbLongSide), "-q:v", "4", poster); err != nil {
		return err
	}
	t, err := describe(poster, "image/jpeg", ".jpg")
	if err != nil {
		return err
	}
	res.Thumb = &t
	return nil
}

func describe(path, contentType, ext string) (File, error) {
	st, err := os.Stat(path)
	if err != nil {
		return File{}, err
	}
	return File{Path: path, ContentType: contentType, Ext: ext, Size: st.Size()}, nil
}

// run executes a tool with a hard timeout and returns its stdout. A non-zero exit becomes a
// toolError (the input is probably bad); a cancelled context or a missing binary stays a plain
// error so the job retries.
func (p *Processor) run(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 5 * time.Second
	// Refuse libvips loaders it considers unsafe for untrusted input (SVG, PDF, ImageMagick...).
	cmd.Env = append(os.Environ(), "VIPS_BLOCK_UNTRUSTED=1")
	var stdout, stderr limitedBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("media: %s: %w", filepath.Base(name), ctx.Err())
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, &toolError{tool: filepath.Base(name), stderr: strings.TrimSpace(stderr.String())}
		}
		return nil, fmt.Errorf("media: run %s: %w", name, err)
	}
	return stdout.b, nil
}

// toolError is a tool refusing its input.
type toolError struct{ tool, stderr string }

func (e *toolError) Error() string { return fmt.Sprintf("media: %s failed: %s", e.tool, e.stderr) }

// bad turns a toolError into the rejection the uploader sees; other errors pass through.
func bad(err error) error {
	var te *toolError
	if errors.As(err, &te) {
		return reject("File tidak bisa dibaca atau rusak.")
	}
	return err
}

func (p *Processor) vips(ctx context.Context, src, out string, side int) error {
	_, err := p.run(ctx, vipsTimeout, p.tools.Vips, "thumbnail", src, out, strconv.Itoa(side), "--size", "down")
	return bad(err)
}

func (p *Processor) vipsDims(ctx context.Context, path string) (w, h int, err error) {
	field := func(name string) (int, error) {
		out, err := p.run(ctx, probeTimeout, p.tools.VipsHeader, "-f", name, path)
		if err != nil {
			return 0, bad(err)
		}
		return strconv.Atoi(strings.TrimSpace(string(out)))
	}
	if w, err = field("width"); err != nil {
		return 0, 0, err
	}
	h, err = field("height")
	return w, h, err
}

func (p *Processor) ffmpeg(ctx context.Context, args ...string) error {
	base := []string{"-hide_banner", "-nostdin", "-v", "error", "-y"}
	// The output path is the last argument; everything before it is options and inputs.
	_, err := p.run(ctx, ffmpegTimeout, p.tools.FFmpeg, append(base, args...)...)
	return bad(err)
}

// limitedBuffer keeps the first 64 KB of a tool's output; a runaway tool must not fill memory.
type limitedBuffer struct{ b []byte }

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if room := 64<<10 - len(l.b); room > 0 {
		l.b = append(l.b, p[:min(room, len(p))]...)
	}
	return len(p), nil
}

func (l *limitedBuffer) String() string { return string(l.b) }
