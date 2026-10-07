package http

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisStorage adapts go-redis to fiber.Storage so Fiber's limiter counts requests
// in Redis and every API instance shares one budget. It borrows the app's client,
// so Close does nothing.
type redisStorage struct {
	rdb    *redis.Client
	prefix string
}

func newRedisStorage(rdb *redis.Client, prefix string) *redisStorage {
	return &redisStorage{rdb: rdb, prefix: prefix}
}

func (s *redisStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	v, err := s.rdb.Get(ctx, s.prefix+key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return v, err
}

func (s *redisStorage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

func (s *redisStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	if key == "" || len(val) == 0 {
		return nil
	}
	return s.rdb.Set(ctx, s.prefix+key, val, exp).Err()
}

func (s *redisStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}

func (s *redisStorage) DeleteWithContext(ctx context.Context, key string) error {
	return s.rdb.Del(ctx, s.prefix+key).Err()
}

func (s *redisStorage) Delete(key string) error {
	return s.DeleteWithContext(context.Background(), key)
}

func (s *redisStorage) ResetWithContext(ctx context.Context) error {
	iter := s.rdb.Scan(ctx, 0, s.prefix+"*", 200).Iterator()
	for iter.Next(ctx) {
		if err := s.rdb.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

func (s *redisStorage) Reset() error { return s.ResetWithContext(context.Background()) }

func (s *redisStorage) Close() error { return nil }
