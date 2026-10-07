package config

import (
	"strings"
	"testing"
)

func validEnv() map[string]string {
	return map[string]string{
		"APP_ENV":        "dev",
		"DATABASE_URL":   "postgres://u:p@localhost:55432/tepati",
		"REDIS_URL":      "redis://localhost:6379/0",
		"SESSION_SECRET": strings.Repeat("a", 64),
		"S3_ACCESS_KEY":  "GKabc",
		"S3_SECRET_KEY":  "secret",
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := LoadFrom(validEnv())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIPort != 8080 || cfg.Storage.Driver != "s3" || cfg.Upload.ImageMaxBytes != 15<<20 {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
	if cfg.WorkerConcurrency != 10 || cfg.WorkerMetricsPort != 9091 {
		t.Fatalf("worker defaults not applied: %+v", cfg)
	}
	if cfg.Upload.VideoMaxSeconds != 180 || cfg.Storage.BucketStaging != "tepati-staging" {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
}

func TestLoadListsEveryMissingVariable(t *testing.T) {
	env := validEnv()
	delete(env, "DATABASE_URL")
	delete(env, "REDIS_URL")
	_, err := LoadFrom(env)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"DATABASE_URL", "REDIS_URL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestValidateRules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]string)
		want   string
	}{
		{"bad env", func(e map[string]string) { e["APP_ENV"] = "prod" }, "APP_ENV"},
		{"short secret", func(e map[string]string) { e["SESSION_SECRET"] = "short" }, "SESSION_SECRET"},
		{"s3 without keys", func(e map[string]string) { delete(e, "S3_ACCESS_KEY") }, "S3_ACCESS_KEY"},
		{"bad driver", func(e map[string]string) { e["STORAGE_DRIVER"] = "ftp" }, "STORAGE_DRIVER"},
		{"bad upload mode", func(e map[string]string) { e["UPLOAD_MODE"] = "magic" }, "UPLOAD_MODE"},
		{"zero worker concurrency", func(e map[string]string) { e["WORKER_CONCURRENCY"] = "0" }, "WORKER_CONCURRENCY"},
		{"worker metrics port out of range", func(e map[string]string) { e["WORKER_METRICS_PORT"] = "70000" }, "WORKER_METRICS_PORT"},
		{"clock override in production", func(e map[string]string) {
			e["APP_ENV"] = "production"
			e["CLOCK_OVERRIDE"] = "true"
		}, "CLOCK_OVERRIDE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			tc.mutate(env)
			_, err := LoadFrom(env)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error mentioning %s, got %v", tc.want, err)
			}
		})
	}
}

func TestFSDriverDoesNotNeedS3Keys(t *testing.T) {
	env := validEnv()
	delete(env, "S3_ACCESS_KEY")
	delete(env, "S3_SECRET_KEY")
	env["STORAGE_DRIVER"] = "fs"
	if _, err := LoadFrom(env); err != nil {
		t.Fatalf("fs driver should not require S3 keys: %v", err)
	}
}
