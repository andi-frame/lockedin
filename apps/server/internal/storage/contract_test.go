package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// runContract is the behaviour every BlobStore must have, whatever it stores in. The fs driver
// runs it in the unit tests; the s3 driver runs it against Garage (integration tag), which is
// where PLAN 4.1's "does Garage enforce the signed length" question gets answered.
func runContract(t *testing.T, s BlobStore) {
	t.Helper()
	ctx := context.Background()
	key := func() string { return "contract/" + uuid.NewString() + ".bin" }
	payload := []byte("catatan belajar: integral parsial")

	put := func(t *testing.T, p PresignedPut, body []byte, mutate func(*http.Request)) *http.Response {
		t.Helper()
		req, err := http.NewRequest(p.Method, p.URL, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range p.Headers {
			req.Header.Set(k, v)
		}
		if mutate != nil {
			mutate(req)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = resp.Body.Close() })
		return resp
	}

	t.Run("put, head, get and delete round-trip", func(t *testing.T) {
		k := key()
		if err := s.Put(ctx, Staging, k, bytes.NewReader(payload), int64(len(payload)), "text/plain"); err != nil {
			t.Fatal(err)
		}
		info, err := s.Head(ctx, Staging, k)
		if err != nil || info.Size != int64(len(payload)) || !strings.HasPrefix(info.ContentType, "text/plain") {
			t.Fatalf("head = %+v, %v", info, err)
		}
		rc, err := s.Get(ctx, Staging, k)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(rc)
		_ = rc.Close()
		if !bytes.Equal(got, payload) {
			t.Fatalf("get = %q", got)
		}
		if err := s.Delete(ctx, Staging, k); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Head(ctx, Staging, k); !errors.Is(err, ErrNotFound) {
			t.Fatalf("head after delete: %v", err)
		}
		if _, err := s.Get(ctx, Staging, k); !errors.Is(err, ErrNotFound) {
			t.Fatalf("get after delete: %v", err)
		}
	})

	t.Run("deleting a missing object is not an error", func(t *testing.T) {
		if err := s.Delete(ctx, Staging, key()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("buckets are separate", func(t *testing.T) {
		k := key()
		if err := s.Put(ctx, Staging, k, bytes.NewReader(payload), int64(len(payload)), "text/plain"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Delete(ctx, Staging, k) })
		if _, err := s.Head(ctx, Media, k); !errors.Is(err, ErrNotFound) {
			t.Fatalf("an object in staging showed up in media: %v", err)
		}
	})

	t.Run("a presigned PUT stores the object", func(t *testing.T) {
		k := key()
		p, err := s.PresignPut(ctx, Staging, k, int64(len(payload)), "image/webp", time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Delete(ctx, Staging, k) })
		if resp := put(t, p, payload, nil); resp.StatusCode/100 != 2 {
			t.Fatalf("PUT = %d", resp.StatusCode)
		}
		info, err := s.Head(ctx, Staging, k)
		if err != nil || info.Size != int64(len(payload)) {
			t.Fatalf("head = %+v, %v", info, err)
		}
	})

	// PLAN 4.1 / ADR-0005: the signature must pin the declared length, so a client cannot
	// declare 1 MB and upload 200 MB.
	t.Run("TestPresignedPutRejectsWrongLength", func(t *testing.T) {
		for name, body := range map[string][]byte{
			"longer":  append(append([]byte{}, payload...), "tambahan"...),
			"shorter": payload[:len(payload)-5],
		} {
			t.Run(name, func(t *testing.T) {
				k := key()
				p, err := s.PresignPut(ctx, Staging, k, int64(len(payload)), "image/webp", time.Minute)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = s.Delete(ctx, Staging, k) })
				resp := put(t, p, body, nil)
				t.Logf("%s body: status %d", name, resp.StatusCode)
				if resp.StatusCode/100 == 2 {
					t.Fatalf("a %d byte body against a %d byte signature was accepted (%d)", len(body), len(payload), resp.StatusCode)
				}
				if _, err := s.Head(ctx, Staging, k); !errors.Is(err, ErrNotFound) {
					t.Fatalf("the rejected upload left an object behind: %v", err)
				}
			})
		}
	})

	t.Run("a presigned PUT rejects another content type", func(t *testing.T) {
		k := key()
		p, err := s.PresignPut(ctx, Staging, k, int64(len(payload)), "image/webp", time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Delete(ctx, Staging, k) })
		resp := put(t, p, payload, func(r *http.Request) { r.Header.Set("Content-Type", "text/html") })
		if resp.StatusCode/100 == 2 {
			t.Fatalf("a different content type was accepted (%d)", resp.StatusCode)
		}
	})

	t.Run("an expired presigned PUT is refused", func(t *testing.T) {
		k := key()
		p, err := s.PresignPut(ctx, Staging, k, int64(len(payload)), "image/webp", time.Second)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Delete(ctx, Staging, k) })
		time.Sleep(2500 * time.Millisecond)
		if resp := put(t, p, payload, nil); resp.StatusCode/100 == 2 {
			t.Fatalf("an expired URL was accepted (%d)", resp.StatusCode)
		}
	})

	t.Run("a presigned GET serves the object and expires", func(t *testing.T) {
		k := key()
		if err := s.Put(ctx, Media, k, bytes.NewReader(payload), int64(len(payload)), "text/plain"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Delete(ctx, Media, k) })
		u, err := s.PresignGet(ctx, Media, k, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.Get(u)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != 200 || !bytes.Equal(got, payload) {
			t.Fatalf("GET = %d %q", resp.StatusCode, got)
		}

		short, err := s.PresignGet(ctx, Media, k, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(2500 * time.Millisecond)
		resp, err = http.Get(short)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode/100 == 2 {
			t.Fatalf("an expired GET URL was accepted (%d)", resp.StatusCode)
		}
	})

	t.Run("keys that could escape the bucket are refused", func(t *testing.T) {
		for _, k := range []string{"", "../x", "a/../../x", "/abs", "a//b", "a\\b", "a b", "a:b", strings.Repeat("a", 600)} {
			if err := s.Put(ctx, Staging, k, bytes.NewReader(payload), int64(len(payload)), "text/plain"); !errors.Is(err, ErrInvalidKey) {
				t.Errorf("Put(%q) = %v, want ErrInvalidKey", k, err)
			}
			if _, err := s.PresignPut(ctx, Staging, k, 1, "text/plain", time.Minute); !errors.Is(err, ErrInvalidKey) {
				t.Errorf("PresignPut(%q) = %v, want ErrInvalidKey", k, err)
			}
			if _, err := s.Head(ctx, Staging, k); !errors.Is(err, ErrInvalidKey) {
				t.Errorf("Head(%q) = %v, want ErrInvalidKey", k, err)
			}
		}
	})
}
