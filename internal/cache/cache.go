// Package cache provides a Redis-backed cache client used for
// frequently-read, large payloads (currently: diagram content).
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNotFound is returned by Get when the key does not exist in cache.
var ErrNotFound = errors.New("cache: not found")

// Client is the cache interface used by handlers.
type Client interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

// DiagramContentKey returns the cache key for a diagram's current content.
func DiagramContentKey(diagramID string) string {
	return "diagram:content:" + diagramID
}

// ActorKey returns the cache key for a resolved actor (user or service account).
func ActorKey(id string) string {
	return "actor:" + id
}

type redisClient struct {
	rc *redis.Client
}

// New creates a Redis cache client from a redis:// or rediss:// URL.
func New(redisURL string) (Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	rc := redis.NewClient(opts)
	return &redisClient{rc: rc}, nil
}

func (c *redisClient) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rc.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

func (c *redisClient) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.rc.Set(ctx, key, value, ttl).Err()
}

func (c *redisClient) Del(ctx context.Context, key string) error {
	return c.rc.Del(ctx, key).Err()
}
