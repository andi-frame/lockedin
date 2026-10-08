package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// PutStagingCORS lets the browser PUT presigned uploads to the staging bucket from the app's
// origin (ADR-0005). It replaces the bucket's CORS rules, so running it again is a no-op.
// The origin must be exact: an empty one is refused rather than widened to "*".
func (s *S3) PutStagingCORS(ctx context.Context, origin string) error {
	if origin == "" {
		return errors.New("storage: cors needs the app origin (APP_BASE_URL)")
	}
	_, err := s.ops.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket: aws.String(s.buckets[Staging]),
		CORSConfiguration: &types.CORSConfiguration{CORSRules: []types.CORSRule{{
			AllowedOrigins: []string{origin},
			AllowedMethods: []string{"PUT", "GET", "HEAD"},
			AllowedHeaders: []string{"*"},
			ExposeHeaders:  []string{"ETag"},
			MaxAgeSeconds:  aws.Int32(3600),
		}}},
	})
	if err != nil {
		return fmt.Errorf("storage: put cors: %w", err)
	}
	return nil
}
