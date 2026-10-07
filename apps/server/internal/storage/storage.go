// Package storage is where uploaded files live (ADR-0005). BlobStore hides whether that is
// Garage (the s3 driver, production and Docker dev) or a local directory (the fs driver, native
// dev without Docker). Nothing outside this package knows which one is running.
package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
)

// Bucket names a logical bucket. The s3 driver maps each to its configured bucket name.
type Bucket string

const (
	Staging Bucket = "staging" // raw uploads, deleted after processing or 24 h
	Media   Bucket = "media"   // processed files, private, served by short-lived signed URLs
)

func (b Bucket) valid() bool { return b == Staging || b == Media }

var (
	ErrNotFound   = errors.New("storage: object not found")
	ErrInvalidKey = errors.New("storage: invalid key")
)

// Info describes a stored object.
type Info struct {
	Size        int64
	ContentType string
}

// PresignedPut is what the browser needs to upload straight to storage. The signature pins the
// object's length and content type, so the headers must be sent as given (the browser adds
// Content-Length itself).
type PresignedPut struct {
	Method  string
	URL     string
	Headers map[string]string
	Expires time.Time
}

// BlobStore is the storage interface of ARCHITECTURE §6.
type BlobStore interface {
	// PresignPut returns a URL for a single PUT of exactly size bytes of contentType.
	PresignPut(ctx context.Context, b Bucket, key string, size int64, contentType string, ttl time.Duration) (PresignedPut, error)
	// PresignGet returns a short-lived URL that reads the object. Nothing is ever public.
	PresignGet(ctx context.Context, b Bucket, key string, ttl time.Duration) (string, error)
	// Head reports an object's size and content type, or ErrNotFound.
	Head(ctx context.Context, b Bucket, key string) (Info, error)
	// Get opens an object for reading, or ErrNotFound. The caller closes it.
	Get(ctx context.Context, b Bucket, key string) (io.ReadCloser, error)
	// Put stores exactly size bytes read from r.
	Put(ctx context.Context, b Bucket, key string, r io.Reader, size int64, contentType string) error
	// Delete removes an object. Deleting one that does not exist is not an error.
	Delete(ctx context.Context, b Bucket, key string) error
}

const maxKeyLen = 512

// ValidKey allows only what our own code generates ("{pact}/{id}.webp", "staging/{id}"):
// letters, digits, '.', '_', '-' and single '/' separators, with no '.' or '..' segment. The
// strictness is the path-traversal defence for the fs driver and keeps keys boring for S3.
func ValidKey(key string) bool {
	if key == "" || len(key) > maxKeyLen {
		return false
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
		for _, r := range seg {
			ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-'
			if !ok {
				return false
			}
		}
	}
	return true
}

func check(b Bucket, key string) error {
	if !b.valid() {
		return errors.New("storage: unknown bucket " + string(b))
	}
	if !ValidKey(key) {
		return ErrInvalidKey
	}
	return nil
}
