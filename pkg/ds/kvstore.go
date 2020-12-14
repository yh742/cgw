package ds

import (
	"context"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// KeyValueStore is interface all db will implement
type KeyValueStore interface {
	Get(context.Context, string) (string, error)
	Set(context.Context, string, string) error
	SetIfNotExist(context.Context, string, string) (bool, error)
}

// RedisStore represents redis storage
type RedisStore struct {
	redisClient *redis.Client
}

// NewRedisStore creates a new RedisStore instance
func NewRedisStore(settings RedisSettings) (RedisStore, error) {
	creds, err := FileCredentials(settings.AuthFile)
	if err != nil {
		return RedisStore{}, err
	}
	rdb := RedisStore{
		redisClient: redis.NewClient(&redis.Options{
			Addr:     settings.Server,
			Username: creds.user,
			Password: creds.password,
			DB:       settings.DBIndex,
		}),
	}

	// make sure connection is up
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	pong, err := rdb.redisClient.Ping(ctx).Result()
	if err != nil && strings.ToLower(pong) == "pong" {
		return RedisStore{}, nil
	}
	return rdb, nil
}

// Get returns a string of the value stored
func (rs RedisStore) Get(ctx context.Context, key string) (string, error) {
	return rs.redisClient.Get(ctx, key).Result()
}

// Set returns set the value of the key to value
func (rs RedisStore) Set(ctx context.Context, key string, value string) error {
	return rs.redisClient.Set(ctx, key, value, 0).Err()
}

// SetIfNotExist sets the value if the key value doesn't exist in a atomic fashion
func (rs RedisStore) SetIfNotExist(ctx context.Context, key string, value string) (bool, error) {
	return rs.redisClient.SetNX(ctx, key, value, 0).Result()
}
