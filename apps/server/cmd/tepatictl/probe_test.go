package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// `tepatictl probe <url>` is the container health check: the images have no curl.
func TestProbe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusFound)
		case "/slow":
			time.Sleep(300 * time.Millisecond)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()

	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"2xx is healthy", []string{"probe", srv.URL + "/ok"}, ""},
		{"a redirect is not followed and is not healthy", []string{"probe", srv.URL + "/redirect"}, "302"},
		{"5xx is unhealthy", []string{"probe", srv.URL + "/down"}, "503"},
		{"nothing listening", []string{"probe", "http://127.0.0.1:1/"}, "probe"},
		{"no url", []string{"probe"}, "tepatictl probe"},
		{"not an http url", []string{"probe", "ftp://x"}, "http"},
		{"timeout", []string{"probe", "--timeout", "50ms", srv.URL + "/slow"}, "probe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := run(context.Background(), tc.args, &out)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("want healthy, got %v", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Fatalf("want an error mentioning %q, got %v", tc.wantErr, err)
			}
		})
	}
}
