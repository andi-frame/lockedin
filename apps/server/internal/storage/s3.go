package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3Config configures the s3 driver. Endpoint is what the server itself connects to;
// PublicEndpoint is the host baked into presigned URLs, because the browser may reach Garage
// under another name than the container network does (ARCHITECTURE §6, RUNNING §4).
type S3Config struct {
	Endpoint       string
	PublicEndpoint string
	Region         string
	AccessKey      string
	SecretKey      string
	StagingBucket  string
	MediaBucket    string
}

// S3 is the BlobStore for Garage (or any S3-compatible server), in path-style addressing.
type S3 struct {
	ops     *s3.Client
	presign *s3.PresignClient
	buckets map[Bucket]string
}

func NewS3(c S3Config) (*S3, error) {
	if c.Endpoint == "" || c.AccessKey == "" || c.SecretKey == "" || c.StagingBucket == "" || c.MediaBucket == "" {
		return nil, errors.New("storage: s3 needs an endpoint, access and secret keys and both bucket names")
	}
	if c.PublicEndpoint == "" {
		c.PublicEndpoint = c.Endpoint
	}
	if c.Region == "" {
		c.Region = "garage"
	}
	client := func(endpoint string) *s3.Client {
		return s3.New(s3.Options{
			Region:       c.Region,
			BaseEndpoint: aws.String(endpoint),
			UsePathStyle: true,
			Credentials:  credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, ""),
			// The SDK's newer default adds CRC32 checksums to every request. Garage does not need
			// them and a presigned URL must not demand a header the browser cannot compute.
			RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
			ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
		})
	}
	return &S3{
		ops:     client(c.Endpoint),
		presign: s3.NewPresignClient(client(c.PublicEndpoint)),
		buckets: map[Bucket]string{Staging: c.StagingBucket, Media: c.MediaBucket},
	}, nil
}

func (s *S3) PresignPut(ctx context.Context, b Bucket, key string, size int64, contentType string, ttl time.Duration) (PresignedPut, error) {
	if err := check(b, key); err != nil {
		return PresignedPut{}, err
	}
	req, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.buckets[b]),
		Key:           aws.String(key),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return PresignedPut{}, fmt.Errorf("storage: presign put: %w", err)
	}
	// Only the headers the client has to send itself. Host comes from the URL and the browser
	// sets Content-Length from the body it uploads.
	headers := map[string]string{}
	for k, v := range req.SignedHeader {
		switch strings.ToLower(k) {
		case "host", "content-length":
		default:
			headers[k] = strings.Join(v, ",")
		}
	}
	return PresignedPut{Method: req.Method, URL: req.URL, Headers: headers, Expires: time.Now().Add(ttl)}, nil
}

func (s *S3) PresignGet(ctx context.Context, b Bucket, key string, ttl time.Duration) (string, error) {
	if err := check(b, key); err != nil {
		return "", err
	}
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.buckets[b]), Key: aws.String(key)}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("storage: presign get: %w", err)
	}
	return req.URL, nil
}

func (s *S3) Head(ctx context.Context, b Bucket, key string) (Info, error) {
	if err := check(b, key); err != nil {
		return Info{}, err
	}
	out, err := s.ops.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.buckets[b]), Key: aws.String(key)})
	if err != nil {
		return Info{}, mapErr(err)
	}
	return Info{Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType)}, nil
}

func (s *S3) Get(ctx context.Context, b Bucket, key string) (io.ReadCloser, error) {
	if err := check(b, key); err != nil {
		return nil, err
	}
	out, err := s.ops.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.buckets[b]), Key: aws.String(key)})
	if err != nil {
		return nil, mapErr(err)
	}
	return out.Body, nil
}

func (s *S3) Put(ctx context.Context, b Bucket, key string, r io.Reader, size int64, contentType string) error {
	if err := check(b, key); err != nil {
		return err
	}
	// Signing the payload over plain HTTP needs a seekable body; spool anything else to disk.
	body, done, err := seekable(r, size)
	if err != nil {
		return err
	}
	defer done()
	_, err = s.ops.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.buckets[b]), Key: aws.String(key),
		Body: body, ContentLength: aws.Int64(size), ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("storage: put: %w", err)
	}
	return nil
}

func (s *S3) Delete(ctx context.Context, b Bucket, key string) error {
	if err := check(b, key); err != nil {
		return err
	}
	// S3 answers 204 for a missing key, so this is already idempotent.
	if _, err := s.ops.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.buckets[b]), Key: aws.String(key)}); err != nil {
		return fmt.Errorf("storage: delete: %w", err)
	}
	return nil
}

// seekable returns r as an io.ReadSeeker holding exactly size bytes.
func seekable(r io.Reader, size int64) (io.ReadSeeker, func(), error) {
	if rs, ok := r.(io.ReadSeeker); ok {
		return io.NewSectionReader(readerAtOf(rs), 0, size), func() {}, nil
	}
	tmp, err := os.CreateTemp("", "tepati-put-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }
	n, err := io.Copy(tmp, io.LimitReader(r, size+1))
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	if n != size {
		cleanup()
		return nil, nil, errWrongLength
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, nil, err
	}
	return tmp, cleanup, nil
}

// readerAtOf adapts a ReadSeeker that is not an io.ReaderAt, so a SectionReader can cap it at size.
func readerAtOf(rs io.ReadSeeker) io.ReaderAt {
	if ra, ok := rs.(io.ReaderAt); ok {
		return ra
	}
	return seekerAt{rs}
}

type seekerAt struct{ rs io.ReadSeeker }

func (s seekerAt) ReadAt(p []byte, off int64) (int, error) {
	if _, err := s.rs.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	return io.ReadFull(s.rs, p)
}

func mapErr(err error) error {
	var nf *types.NotFound
	var nk *types.NoSuchKey
	if errors.As(err, &nf) || errors.As(err, &nk) {
		return ErrNotFound
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		switch api.ErrorCode() {
		case "NotFound", "NoSuchKey", "NoSuchBucket":
			return ErrNotFound
		}
	}
	var re interface{ HTTPStatusCode() int }
	if errors.As(err, &re) && re.HTTPStatusCode() == http.StatusNotFound {
		return ErrNotFound
	}
	return fmt.Errorf("storage: %w", err)
}
