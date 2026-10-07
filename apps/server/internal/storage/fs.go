package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// BlobPrefix is where the API mounts the fs driver's handler.
const BlobPrefix = "/api/v1/blob/"

// FS keeps objects in a local directory and hands out HMAC-signed URLs that its own ServeHTTP
// verifies. It exists so native dev works without Docker; it is not for production.
type FS struct {
	dir    string
	secret []byte
	base   string // public origin of the API, e.g. http://localhost:8080
	now    func() time.Time
}

func NewFS(dir, secret, publicBase string) (*FS, error) {
	switch {
	case dir == "":
		return nil, errors.New("storage: fs directory is empty")
	case len(secret) < 16:
		return nil, errors.New("storage: fs signing secret must be at least 16 characters")
	case publicBase == "":
		return nil, errors.New("storage: fs public base URL is empty")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	return &FS{dir: abs, secret: []byte(secret), base: strings.TrimRight(publicBase, "/"), now: time.Now}, nil
}

func (f *FS) path(b Bucket, key string) string {
	return filepath.Join(f.dir, string(b), filepath.FromSlash(key))
}
func (f *FS) metaPath(b Bucket, key string) string {
	return filepath.Join(f.dir, ".meta", string(b), filepath.FromSlash(key))
}

func (f *FS) sign(method string, b Bucket, key string, exp int64, size int64, ct string) string {
	m := hmac.New(sha256.New, f.secret)
	fmt.Fprintf(m, "%s\n%s\n%s\n%d\n%d\n%s", method, b, key, exp, size, ct)
	return hex.EncodeToString(m.Sum(nil))
}

func (f *FS) signedURL(method string, b Bucket, key string, size int64, ct string, ttl time.Duration) (string, time.Time) {
	expires := f.now().Add(ttl)
	exp := expires.Unix()
	q := url.Values{}
	q.Set("m", method)
	q.Set("exp", strconv.FormatInt(exp, 10))
	if method == http.MethodPut {
		q.Set("len", strconv.FormatInt(size, 10))
		q.Set("ct", ct)
	}
	q.Set("sig", f.sign(method, b, key, exp, size, ct))
	return f.base + BlobPrefix + string(b) + "/" + key + "?" + q.Encode(), expires
}

func (f *FS) PresignPut(_ context.Context, b Bucket, key string, size int64, contentType string, ttl time.Duration) (PresignedPut, error) {
	if err := check(b, key); err != nil {
		return PresignedPut{}, err
	}
	u, exp := f.signedURL(http.MethodPut, b, key, size, contentType, ttl)
	return PresignedPut{Method: http.MethodPut, URL: u, Headers: map[string]string{"Content-Type": contentType}, Expires: exp}, nil
}

func (f *FS) PresignGet(_ context.Context, b Bucket, key string, ttl time.Duration) (string, error) {
	if err := check(b, key); err != nil {
		return "", err
	}
	u, _ := f.signedURL(http.MethodGet, b, key, 0, "", ttl)
	return u, nil
}

func (f *FS) Head(_ context.Context, b Bucket, key string) (Info, error) {
	if err := check(b, key); err != nil {
		return Info{}, err
	}
	st, err := os.Stat(f.path(b, key))
	if errors.Is(err, os.ErrNotExist) {
		return Info{}, ErrNotFound
	}
	if err != nil {
		return Info{}, err
	}
	ct, _ := os.ReadFile(f.metaPath(b, key))
	return Info{Size: st.Size(), ContentType: string(ct)}, nil
}

func (f *FS) Get(_ context.Context, b Bucket, key string) (io.ReadCloser, error) {
	if err := check(b, key); err != nil {
		return nil, err
	}
	file, err := os.Open(f.path(b, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return file, err
}

func (f *FS) Put(_ context.Context, b Bucket, key string, r io.Reader, size int64, contentType string) error {
	if err := check(b, key); err != nil {
		return err
	}
	return f.write(b, key, r, size, contentType)
}

func (f *FS) Delete(_ context.Context, b Bucket, key string) error {
	if err := check(b, key); err != nil {
		return err
	}
	for _, p := range []string{f.path(b, key), f.metaPath(b, key)} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

var errWrongLength = errors.New("storage: body length does not match the declared size")

// write stores exactly size bytes. It writes a temp file first, so a half-sent upload never
// shows up as an object.
func (f *FS) write(b Bucket, key string, r io.Reader, size int64, ct string) error {
	tmpDir := filepath.Join(f.dir, ".tmp")
	if err := os.MkdirAll(tmpDir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(tmpDir, "up-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	n, err := io.Copy(tmp, io.LimitReader(r, size+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n != size {
		return errWrongLength
	}
	dst := f.path(b, key)
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	meta := f.metaPath(b, key)
	if err := os.MkdirAll(filepath.Dir(meta), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(meta, []byte(ct), 0o640); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

// ServeHTTP serves signed URLs. Mount it at BlobPrefix. Every failure to verify is a 403 with no
// detail, so the response does not help anyone guess a signature.
func (f *FS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	deny := func() { http.Error(w, "forbidden", http.StatusForbidden) }
	rest, ok := strings.CutPrefix(r.URL.Path, BlobPrefix)
	if !ok {
		http.NotFound(w, r)
		return
	}
	bucketName, key, ok := strings.Cut(rest, "/")
	b := Bucket(bucketName)
	if !ok || check(b, key) != nil {
		deny()
		return
	}
	q := r.URL.Query()
	method := q.Get("m")
	exp, err := strconv.ParseInt(q.Get("exp"), 10, 64)
	if err != nil || method != r.Method || f.now().Unix() > exp {
		deny()
		return
	}
	var size int64
	ct := ""
	if method == http.MethodPut {
		if size, err = strconv.ParseInt(q.Get("len"), 10, 64); err != nil || size < 0 {
			deny()
			return
		}
		ct = q.Get("ct")
	}
	if !hmac.Equal([]byte(q.Get("sig")), []byte(f.sign(method, b, key, exp, size, ct))) {
		deny()
		return
	}

	switch method {
	case http.MethodGet:
		info, err := f.Head(r.Context(), b, key)
		if errors.Is(err, ErrNotFound) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}
		file, err := f.Get(r.Context(), b, key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		if info.ContentType != "" {
			w.Header().Set("Content-Type", info.ContentType)
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
		_, _ = io.Copy(w, file)
	case http.MethodPut:
		// The same two checks an S3 signature makes: length and content type are part of what was signed.
		if r.ContentLength != size || r.Header.Get("Content-Type") != ct {
			http.Error(w, "length or content type does not match the signature", http.StatusBadRequest)
			return
		}
		if err := f.write(b, key, r.Body, size, ct); err != nil {
			if errors.Is(err, errWrongLength) {
				http.Error(w, "body length does not match", http.StatusBadRequest)
				return
			}
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
