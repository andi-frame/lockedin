package storage

import (
	"testing"

	"github.com/andi-frame/lockedin/apps/server/internal/config"
)

func TestFromConfigPicksTheDriver(t *testing.T) {
	fs, err := FromConfig(config.Config{
		APIPort: 8080, SessionSecret: "0123456789abcdef0123456789abcdef",
		Storage: config.Storage{Driver: "fs", FSDir: t.TempDir()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fs.(*FS); !ok {
		t.Fatalf("fs driver gave %T", fs)
	}

	s3, err := FromConfig(config.Config{Storage: config.Storage{
		Driver: "s3", Endpoint: "http://garage:3900", AccessKey: "k", SecretKey: "s", BucketStaging: "a", BucketMedia: "b",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s3.(*S3); !ok {
		t.Fatalf("s3 driver gave %T", s3)
	}

	if _, err := FromConfig(config.Config{Storage: config.Storage{Driver: "gcs"}}); err == nil {
		t.Fatal("an unknown driver was accepted")
	}
}

func TestFSURLsPointAtTheAPI(t *testing.T) {
	st, err := FromConfig(config.Config{
		APIPort: 9999, SessionSecret: "0123456789abcdef0123456789abcdef",
		Storage: config.Storage{Driver: "fs", FSDir: t.TempDir()},
	})
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.PresignGet(t.Context(), Media, "a/b.webp", 60e9)
	if err != nil {
		t.Fatal(err)
	}
	if want := "http://localhost:9999/api/v1/blob/media/a/b.webp?"; len(u) < len(want) || u[:len(want)] != want {
		t.Fatalf("url = %s", u)
	}
}
