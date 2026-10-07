package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestFS returns an fs store whose signed URLs point at a live test server running its handler.
func newTestFS(t *testing.T) (*FS, *httptest.Server) {
	t.Helper()
	var fs *FS
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fs.ServeHTTP(w, r) }))
	t.Cleanup(srv.Close)
	fs, err := NewFS(t.TempDir(), "test-secret-test-secret", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return fs, srv
}

func TestFSMeetsTheBlobStoreContract(t *testing.T) {
	fs, _ := newTestFS(t)
	runContract(t, fs)
}

func TestFSRejectsATamperedSignature(t *testing.T) {
	fs, _ := newTestFS(t)
	u, err := fs.PresignGet(context.Background(), Media, "a/b.webp", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutated := range map[string]string{
		"signature": strings.Replace(u, "sig=", "sig=00", 1),
		"expiry":    strings.Replace(u, "exp=", "exp=9", 1),
		"key":       strings.Replace(u, "a/b.webp", "a/c.webp", 1),
		"bucket":    strings.Replace(u, "/media/", "/staging/", 1),
		"no sig":    u[:strings.Index(u, "sig=")],
	} {
		resp, err := http.Get(mutated)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", name, resp.StatusCode)
		}
	}
}

func TestFSGetURLCannotBeUsedToWrite(t *testing.T) {
	fs, _ := newTestFS(t)
	u, err := fs.PresignGet(context.Background(), Media, "a/b.webp", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPut, u, strings.NewReader("x"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("PUT with a GET signature = %d, want 403", resp.StatusCode)
	}
}

func TestNewFSValidatesItsInput(t *testing.T) {
	if _, err := NewFS(t.TempDir(), "short", "http://x"); err == nil {
		t.Error("a short secret was accepted")
	}
	if _, err := NewFS("", "test-secret-test-secret", "http://x"); err == nil {
		t.Error("an empty directory was accepted")
	}
	if _, err := NewFS(t.TempDir(), "test-secret-test-secret", ""); err == nil {
		t.Error("an empty public base URL was accepted")
	}
}
