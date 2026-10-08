package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The browser PUTs a presigned upload straight to the staging bucket from the app's origin, so
// that bucket needs a CORS rule (ADR-0005). The deploy runs this from inside the compose
// network, where Garage's S3 port is not published.
func TestPutStagingCORS(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotMethod, gotPath, gotQuery, gotBody = r.Method, r.URL.Path, r.URL.RawQuery, string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s, err := NewS3(S3Config{Endpoint: srv.URL, AccessKey: "k", SecretKey: "s", StagingBucket: "stage", MediaBucket: "media"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutStagingCORS(context.Background(), "https://app.example"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPut || gotPath != "/stage" || !strings.HasPrefix(gotQuery, "cors") {
		t.Fatalf("want PUT /stage?cors, got %s %s?%s", gotMethod, gotPath, gotQuery)
	}
	for _, want := range []string{"<AllowedOrigin>https://app.example</AllowedOrigin>", "<AllowedMethod>PUT</AllowedMethod>", "<ExposeHeader>ETag</ExposeHeader>"} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("body lacks %s:\n%s", want, gotBody)
		}
	}
}

func TestPutStagingCORSNeedsAnOrigin(t *testing.T) {
	s, err := NewS3(S3Config{Endpoint: "http://127.0.0.1:1", AccessKey: "k", SecretKey: "s", StagingBucket: "stage", MediaBucket: "media"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutStagingCORS(context.Background(), ""); err == nil {
		t.Fatal("an empty origin must be refused, not turned into a wildcard")
	}
}
