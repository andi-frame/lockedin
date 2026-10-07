package storage

import (
	"fmt"

	"github.com/andi-frame/lockedin/apps/server/internal/config"
)

// FromConfig builds the driver STORAGE_DRIVER names. The fs driver signs with SESSION_SECRET
// and hands out URLs on the API itself (it serves them), so it is for native dev only.
func FromConfig(c config.Config) (BlobStore, error) {
	s := c.Storage
	switch s.Driver {
	case "s3":
		return NewS3(S3Config{
			Endpoint: s.Endpoint, PublicEndpoint: s.PublicEndpoint, Region: s.Region,
			AccessKey: s.AccessKey, SecretKey: s.SecretKey,
			StagingBucket: s.BucketStaging, MediaBucket: s.BucketMedia,
		})
	case "fs":
		return NewFS(s.FSDir, c.SessionSecret, fmt.Sprintf("http://localhost:%d", c.APIPort))
	}
	return nil, fmt.Errorf("storage: unknown driver %q", s.Driver)
}
