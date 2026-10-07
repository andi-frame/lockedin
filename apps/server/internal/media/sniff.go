package media

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strconv"
)

// Sniff reads the file's magic bytes and returns the real MIME type and our kind for it. The
// extension and the client's declared type play no part (invariant 7). Anything outside the
// SPEC §8 list is ErrUnsupported.
func Sniff(path string) (mime string, kind Kind, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	head := make([]byte, 64)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", "", err
	}
	head = head[:n]

	switch {
	case bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", KindImage, nil
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", KindImage, nil
	case bytes.HasPrefix(head, []byte("GIF87a")), bytes.HasPrefix(head, []byte("GIF89a")):
		return "image/gif", KindImage, nil
	case len(head) >= 12 && bytes.HasPrefix(head, []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")):
		return "image/webp", KindImage, nil
	case bytes.HasPrefix(head, []byte("%PDF-")):
		return "application/pdf", KindFile, nil
	case bytes.HasPrefix(head, []byte{0x1A, 0x45, 0xDF, 0xA3}) && bytes.Contains(head, []byte("webm")):
		return "video/webm", KindVideo, nil
	case len(head) >= 12 && bytes.Equal(head[4:8], []byte("ftyp")):
		return ftyp(string(head[8:12]))
	}
	return "", "", ErrUnsupported
}

// ftyp classifies an ISO base media file by its major brand.
func ftyp(brand string) (string, Kind, error) {
	switch brand {
	case "heic", "heix", "hevc", "hevx", "mif1", "msf1":
		return "image/heic", KindImage, nil
	case "avif", "avis":
		return "image/avif", KindImage, nil
	case "qt  ":
		return "video/quicktime", KindVideo, nil
	}
	return "video/mp4", KindVideo, nil
}

type probeInfo struct {
	hasVideo      bool
	width, height int
	seconds       float64
}

// probe asks ffprobe about a video. It reads the container only, so it is cheap even for a long file.
func (p *Processor) probe(ctx context.Context, path string) (probeInfo, error) {
	out, err := p.run(ctx, probeTimeout, p.tools.FFprobe,
		"-v", "error", "-protocol_whitelist", "file", "-print_format", "json", "-show_format", "-show_streams", path)
	if err != nil {
		return probeInfo{}, bad(err)
	}
	var doc struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Duration  string `json:"duration"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return probeInfo{}, err
	}
	var info probeInfo
	for _, s := range doc.Streams {
		if s.CodecType == "video" && !info.hasVideo {
			info.hasVideo, info.width, info.height = true, s.Width, s.Height
		}
	}
	info.seconds, _ = strconv.ParseFloat(doc.Format.Duration, 64)
	return info, nil
}
