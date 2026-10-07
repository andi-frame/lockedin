package http

import (
	"context"
	"fmt"
	"io"
	nethttp "net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/andi-frame/lockedin/apps/server/internal/config"
)

func DatabaseCheck(pool *pgxpool.Pool) ReadyCheck {
	return ReadyCheck{Name: "database", Ping: pool.Ping}
}

func RedisCheck(rdb *redis.Client) ReadyCheck {
	return ReadyCheck{Name: "redis", Ping: func(ctx context.Context) error { return rdb.Ping(ctx).Err() }}
}

// StorageCheck is a reachability probe: the fs driver needs a writable directory, and
// the s3 driver needs the endpoint to answer at all (Garage answers an unsigned request
// with an error status, which still proves it is up). PLAN 4.1 can replace it with a
// real BlobStore ping.
func StorageCheck(cfg config.Storage) ReadyCheck {
	return ReadyCheck{Name: "storage", Ping: func(ctx context.Context) error {
		if cfg.Driver == "fs" {
			if err := os.MkdirAll(cfg.FSDir, 0o755); err != nil {
				return fmt.Errorf("fs storage dir: %w", err)
			}
			return nil
		}
		req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, cfg.Endpoint, nil)
		if err != nil {
			return err
		}
		resp, err := nethttp.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("s3 endpoint: %w", err)
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))
		return resp.Body.Close()
	}}
}
