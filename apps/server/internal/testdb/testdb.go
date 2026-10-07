//go:build integration

// Package testdb gives integration tests a throwaway, fully migrated database.
//
// It connects to TEST_DATABASE_URL (or DATABASE_URL, i.e. the dev Postgres from
// `bun run infra:up`), creates tepati_test_<random>, applies every goose "Up"
// section from db/migrations, and drops the database when the test finishes.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// New returns a store on a fresh database. The database is dropped in t.Cleanup.
func New(t testing.TB) *store.Store {
	t.Helper()
	ctx := context.Background()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		adminURL = os.Getenv("DATABASE_URL")
	}
	if adminURL == "" {
		adminURL = readRootEnv("DATABASE_URL")
	}
	if adminURL == "" {
		t.Skip("no TEST_DATABASE_URL/DATABASE_URL; run `bun run infra:up`")
	}

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("testdb: connect admin: %v", err)
	}
	defer admin.Close(ctx)

	buf := make([]byte, 6)
	_, _ = rand.Read(buf)
	name := "tepati_test_" + hex.EncodeToString(buf)
	if _, err := admin.Exec(ctx, "create database "+name); err != nil {
		t.Fatalf("testdb: create database: %v", err)
	}

	u, _ := url.Parse(adminURL)
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("testdb: pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		c, err := pgx.Connect(context.Background(), adminURL)
		if err != nil {
			return
		}
		defer c.Close(context.Background())
		_, _ = c.Exec(context.Background(), "drop database if exists "+name+" with (force)")
	})

	if err := migrate(ctx, pool); err != nil {
		t.Fatalf("testdb: migrate: %v", err)
	}
	return store.NewStore(pool)
}

func serverRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// migrate applies the "-- +goose Up" section of every migration in order.
func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	dir := filepath.Join(serverRoot(), "db", "migrations")
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		up := string(raw)
		if i := strings.Index(up, "-- +goose Up"); i >= 0 {
			up = up[i:]
		}
		if i := strings.Index(up, "-- +goose Down"); i >= 0 {
			up = up[:i]
		}
		if _, err := pool.Exec(ctx, up); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

// Redis returns a client on logical DB 15 of the dev Redis (REDIS_URL), flushed
// before and after the test. Tests in one package must not run Redis tests in parallel.
func Redis(t testing.TB) *redis.Client { t.Helper(); return RedisIn(t, 15) }

// RedisIn is Redis on another logical DB. `go test ./...` runs packages in parallel and
// each flushes its DB, so every package that uses Redis in tests takes its own index
// (auth: 15, http: 14).
func RedisIn(t testing.TB, db int) *redis.Client {
	t.Helper()
	raw := os.Getenv("REDIS_URL")
	if raw == "" {
		raw = readRootEnv("REDIS_URL")
	}
	if raw == "" {
		t.Skip("no REDIS_URL; run `bun run infra:up`")
	}
	opt, err := redis.ParseURL(raw)
	if err != nil {
		t.Fatalf("testdb: redis url: %v", err)
	}
	opt.DB = db
	rdb := redis.NewClient(opt)
	ctx := context.Background()
	if err := rdb.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("testdb: redis: %v", err)
	}
	t.Cleanup(func() {
		_ = rdb.FlushDB(context.Background()).Err()
		_ = rdb.Close()
	})
	return rdb
}

// readRootEnv reads one key from the repo root .env so `go test` works without Bun.
// Setting returns an environment variable, or else its value in the repo's root .env. Tests for
// other infrastructure (Garage keys) use it so they run without exporting anything.
func Setting(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return readRootEnv(key)
}

func readRootEnv(key string) string {
	raw, err := os.ReadFile(filepath.Join(serverRoot(), "..", "..", ".env"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || k != key {
			continue
		}
		if i := strings.Index(v, " #"); i >= 0 {
			v = v[:i]
		}
		return strings.Trim(strings.TrimSpace(v), `"`)
	}
	return ""
}
