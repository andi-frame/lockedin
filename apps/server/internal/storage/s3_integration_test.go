//go:build integration

package storage

import (
	"testing"

	"github.com/andi-frame/lockedin/apps/server/internal/testdb"
)

func newTestS3(t *testing.T) *S3 {
	t.Helper()
	cfg := S3Config{
		Endpoint:       testdb.Setting("S3_ENDPOINT"),
		PublicEndpoint: testdb.Setting("S3_PUBLIC_ENDPOINT"),
		Region:         testdb.Setting("S3_REGION"),
		AccessKey:      testdb.Setting("S3_ACCESS_KEY"),
		SecretKey:      testdb.Setting("S3_SECRET_KEY"),
		StagingBucket:  testdb.Setting("S3_BUCKET_STAGING"),
		MediaBucket:    testdb.Setting("S3_BUCKET_MEDIA"),
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		t.Skip("no S3_ACCESS_KEY/S3_SECRET_KEY; run `bun run infra:up` and `bun run garage:init`")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:3900"
	}
	if cfg.StagingBucket == "" {
		cfg.StagingBucket = "tepati-staging"
	}
	if cfg.MediaBucket == "" {
		cfg.MediaBucket = "tepati-media"
	}
	s, err := NewS3(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// The same contract as the fs driver, against a real Garage. TestPresignedPutRejectsWrongLength
// inside it is the check PLAN 4.1 asks for; its outcome is recorded in ADR-0005.
func TestS3MeetsTheBlobStoreContract(t *testing.T) {
	runContract(t, newTestS3(t))
}
