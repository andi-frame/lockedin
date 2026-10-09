package media

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// The golden inputs are generated here instead of committed: a 12 MP photo and a 1080p clip
// would add megabytes of binary to the repo, and generating them documents exactly what each
// one is. Tests skip, loudly, when the tools are missing (CI installs them, PLAN 8.x).

func testTools(t *testing.T) Tools {
	t.Helper()
	tools := Tools{FFmpeg: "ffmpeg", FFprobe: "ffprobe", Vips: "vips", VipsHeader: "vipsheader"}
	for _, bin := range []string{tools.FFmpeg, tools.FFprobe, tools.Vips, tools.VipsHeader} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s is not on PATH; install ffmpeg and libvips to run the media golden tests", bin)
		}
	}
	return tools
}

func run(t *testing.T, name string, args ...string) {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

// photo12MPWithGPS is a 4000x3000 JPEG carrying a GPS EXIF block (-6.9, 107.6: Bandung).
func photo12MPWithGPS(t *testing.T, dir string) string {
	t.Helper()
	plain := filepath.Join(dir, "plain.jpg")
	run(t, "ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=4000x3000", "-frames:v", "1", "-q:v", "3", plain)
	raw, err := os.ReadFile(plain)
	if err != nil {
		t.Fatal(err)
	}
	exif := gpsExif()
	seg := append([]byte{0xFF, 0xE1}, u16be(2+len(exif))...)
	seg = append(seg, exif...)
	withExif := append(append(append([]byte{}, raw[:2]...), seg...), raw[2:]...)
	out := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(out, withExif, 0o600); err != nil {
		t.Fatal(err)
	}
	return out
}

func u16be(n int) []byte { return []byte{byte(n >> 8), byte(n)} }

// gpsExif builds "Exif\0\0" plus a little-endian TIFF with IFD0 -> GPS IFD (lat 6 54 0 S, lon 107 36 0 E).
func gpsExif() []byte {
	le := binary.LittleEndian
	var b bytes.Buffer
	b.WriteString("II")
	_ = binary.Write(&b, le, uint16(42))
	_ = binary.Write(&b, le, uint32(8))
	entry := func(tag, typ uint16, count, val uint32) {
		_ = binary.Write(&b, le, tag)
		_ = binary.Write(&b, le, typ)
		_ = binary.Write(&b, le, count)
		_ = binary.Write(&b, le, val)
	}
	// IFD0 at 8: orientation + pointer to the GPS IFD at 38
	_ = binary.Write(&b, le, uint16(2))
	entry(0x0112, 3, 1, 1)
	entry(0x8825, 4, 1, 38)
	_ = binary.Write(&b, le, uint32(0))
	// GPS IFD at 38: refs inline, rationals at 92 and 116
	_ = binary.Write(&b, le, uint16(4))
	entry(0x0001, 2, 2, uint32('S'))
	entry(0x0002, 5, 3, 92)
	entry(0x0003, 2, 2, uint32('E'))
	entry(0x0004, 5, 3, 116)
	_ = binary.Write(&b, le, uint32(0))
	for _, v := range [][2]uint32{{6, 1}, {54, 1}, {0, 1}, {107, 1}, {36, 1}, {0, 1}} {
		_ = binary.Write(&b, le, v[0])
		_ = binary.Write(&b, le, v[1])
	}
	return append([]byte("Exif\x00\x00"), b.Bytes()...)
}

func pngFile(t *testing.T, path string) string {
	t.Helper()
	// -f image2 and -c:v png force PNG bytes whatever the file is called: without them ffmpeg picks
	// the container from the extension, and "holiday.mp4" became a real one-frame video, not a renamed PNG.
	run(t, "ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=320x240", "-frames:v", "1", "-f", "image2", "-c:v", "png", path)
	return path
}

// clip makes an H.264 mp4 of the given size and length. Low frame rates keep generation fast.
func clip(t *testing.T, path, size string, rate, seconds int, audio bool) string {
	t.Helper()
	args := []string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=" + size + ":rate=" + strconv.Itoa(rate)}
	if audio {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440")
	}
	args = append(args, "-t", strconv.Itoa(seconds), "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p")
	if audio {
		args = append(args, "-c:a", "aac", "-shortest")
	}
	run(t, "ffmpeg", append(args, path)...)
	return path
}
