// Package config loads and validates process configuration from environment
// variables. Every variable is documented in deploy/env/.env.example.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Env           string `env:"APP_ENV" envDefault:"dev"`
	BaseURL       string `env:"APP_BASE_URL" envDefault:"http://localhost:3000"`
	APIPort       int    `env:"API_PORT" envDefault:"8080"`
	SessionSecret string `env:"SESSION_SECRET,required"`
	ClockOverride bool   `env:"CLOCK_OVERRIDE" envDefault:"false"`
	DatabaseURL   string `env:"DATABASE_URL,required"`
	RedisURL      string `env:"REDIS_URL,required"`
	SMTPURL       string `env:"SMTP_URL" envDefault:"smtp://localhost:1025"`
	MailFrom      string `env:"MAIL_FROM" envDefault:"Tepati <no-reply@tepati.local>"`
	MediaQueueMax int    `env:"MEDIA_QUEUE_MAX" envDefault:"500"`
	// Worker: jobs run at once, and the port of its /metrics listener (0 turns it off).
	WorkerConcurrency int `env:"WORKER_CONCURRENCY" envDefault:"10"`
	WorkerMetricsPort int `env:"WORKER_METRICS_PORT" envDefault:"9091"`
	Storage           Storage
	Upload            Upload
	Media             MediaTools
}

type Storage struct {
	Driver         string `env:"STORAGE_DRIVER" envDefault:"s3"`
	FSDir          string `env:"FS_STORAGE_DIR" envDefault:"./.data/blobs"`
	UploadMode     string `env:"UPLOAD_MODE" envDefault:"presigned"`
	Endpoint       string `env:"S3_ENDPOINT" envDefault:"http://localhost:3900"`
	PublicEndpoint string `env:"S3_PUBLIC_ENDPOINT" envDefault:"http://localhost:3900"`
	Region         string `env:"S3_REGION" envDefault:"garage"`
	AccessKey      string `env:"S3_ACCESS_KEY"`
	SecretKey      string `env:"S3_SECRET_KEY"`
	BucketStaging  string `env:"S3_BUCKET_STAGING" envDefault:"tepati-staging"`
	BucketMedia    string `env:"S3_BUCKET_MEDIA" envDefault:"tepati-media"`
}

// Upload limits, SPEC §8. The server is the authority; the client only pre-checks.
type Upload struct {
	ImageMaxBytes   int64 `env:"UPLOAD_IMAGE_MAX_BYTES" envDefault:"15728640"`
	VideoMaxBytes   int64 `env:"UPLOAD_VIDEO_MAX_BYTES" envDefault:"209715200"`
	VideoMaxSeconds int   `env:"UPLOAD_VIDEO_MAX_SECONDS" envDefault:"180"`
	FileMaxBytes    int64 `env:"UPLOAD_FILE_MAX_BYTES" envDefault:"20971520"`
	MaxPerProof     int   `env:"UPLOAD_MAX_PER_PROOF" envDefault:"10"`
}

type MediaTools struct {
	FFmpeg  string `env:"FFMPEG_PATH" envDefault:"ffmpeg"`
	FFprobe string `env:"FFPROBE_PATH" envDefault:"ffprobe"`
	Vips    string `env:"VIPS_PATH" envDefault:"vips"`
}

func (c Config) IsProduction() bool { return c.Env == "production" }

// Load reads the process environment.
func Load() (Config, error) { return LoadFrom(envMap()) }

// LoadFrom parses the given variables, so tests never depend on the real environment.
func LoadFrom(vars map[string]string) (Config, error) {
	var cfg Config
	if err := env.ParseWithOptions(&cfg, env.Options{Environment: vars}); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}

func (c Config) validate() error {
	var errs []error
	check := func(ok bool, format string, args ...any) {
		if !ok {
			errs = append(errs, fmt.Errorf(format, args...))
		}
	}
	check(oneOf(c.Env, "dev", "staging", "production"), "APP_ENV must be dev, staging or production (got %q)", c.Env)
	check(len(c.SessionSecret) >= 32, "SESSION_SECRET must be at least 32 characters")
	check(oneOf(c.Storage.Driver, "s3", "fs"), "STORAGE_DRIVER must be s3 or fs (got %q)", c.Storage.Driver)
	check(oneOf(c.Storage.UploadMode, "presigned", "proxy"), "UPLOAD_MODE must be presigned or proxy (got %q)", c.Storage.UploadMode)
	if c.Storage.Driver == "s3" {
		check(c.Storage.AccessKey != "", "S3_ACCESS_KEY is required when STORAGE_DRIVER=s3 (run `bun run garage:init`)")
		check(c.Storage.SecretKey != "", "S3_SECRET_KEY is required when STORAGE_DRIVER=s3 (run `bun run garage:init`)")
	}
	// The test clock lets e2e tests move time; it must never be reachable in production.
	check(!(c.ClockOverride && c.IsProduction()), "CLOCK_OVERRIDE must be false in production")
	check(c.APIPort > 0 && c.APIPort < 65536, "API_PORT out of range")
	check(c.WorkerConcurrency > 0, "WORKER_CONCURRENCY must be at least 1")
	check(c.WorkerMetricsPort >= 0 && c.WorkerMetricsPort < 65536, "WORKER_METRICS_PORT out of range")
	return errors.Join(errs...)
}

func oneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

func envMap() map[string]string {
	out := make(map[string]string)
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			out[k] = v
		}
	}
	return out
}
