package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
	"github.com/andi-frame/lockedin/apps/server/internal/storage"
)

func withBlob(h nethttp.Handler, maxBytes int) option {
	return func(d *Deps) { d.Blob, d.BlobMaxBytes = h, maxBytes }
}

func newBlobEnv(t *testing.T) (*env, *storage.FS) {
	t.Helper()
	fs, err := storage.NewFS(t.TempDir(), "test-secret-test-secret", "http://api.test")
	if err != nil {
		t.Fatal(err)
	}
	return newEnv(t, withBlob(fs, 8<<20)), fs
}

func (e *env) raw(t *testing.T, method, target string, body []byte, hdr map[string]string) (int, []byte) {
	t.Helper()
	r := httptest.NewRequest(method, target, bytes.NewReader(body))
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	resp, err := e.app.Test(r, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// pathOf turns a presigned URL into the path-and-query a client request would carry.
func pathOf(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.RequestURI()
}

func TestFSBlobRouteServesSignedUploadsWithoutASession(t *testing.T) {
	e, fs := newBlobEnv(t)
	body := bytes.Repeat([]byte("x"), 3<<20) // bigger than the 1 MB JSON limit
	put, err := fs.PresignPut(context.Background(), storage.Staging, "u/1.bin", int64(len(body)), "image/webp", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	status, msg := e.raw(t, put.Method, pathOf(t, put.URL), body, put.Headers)
	if status != 200 {
		t.Fatalf("signed PUT = %d %s", status, msg)
	}

	get, err := fs.PresignGet(context.Background(), storage.Staging, "u/1.bin", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	status, got := e.raw(t, "GET", pathOf(t, get), nil, nil)
	if status != 200 || !bytes.Equal(got, body) {
		t.Fatalf("signed GET = %d, %d bytes", status, len(got))
	}
}

func TestFSBlobRouteRefusesUnsignedRequests(t *testing.T) {
	e, _ := newBlobEnv(t)
	for _, m := range []string{"GET", "PUT"} {
		status, _ := e.raw(t, m, "/api/v1/blob/staging/u/1.bin", []byte("x"), nil)
		if status != 403 {
			t.Errorf("unsigned %s = %d, want 403", m, status)
		}
	}
}

// Raising Fiber's body limit for the blob route must not raise it for everything else.
func TestJSONRoutesKeepTheOneMegabyteLimitWhenBlobIsMounted(t *testing.T) {
	e, _ := newBlobEnv(t)
	big := `{"email":"` + strings.Repeat("a", 2<<20) + `"}`
	status, raw := e.raw(t, "POST", "/api/v1/auth/login", []byte(big), map[string]string{"Content-Type": "application/json"})
	if status != 413 {
		t.Fatalf("status = %d body %.200s", status, raw)
	}
	var p api.Problem
	if err := json.Unmarshal(raw, &p); err != nil || p.Code != api.RequestTooLarge {
		t.Fatalf("body = %.200s (%v)", raw, err)
	}
}
