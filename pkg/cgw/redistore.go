package ds

import (
	"context"
	"strings"
	"time"

	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v8"
)

// KeyValueStore is interface all db will implement
// type KeyValueStore interface {
// 	HGet(context.Context, string, string) (string, error)
// 	HSet(context.Context, string, ...interface{}) (int64, error)
// 	HSetNX(context.Context, string, string, ...interface{}) (bool, error)
// 	Get(context.Context, string) (string, error)
// 	Set(context.Context, string, string) error
// 	SetIfNotExist(context.Context, string, string) (bool, error)
// 	GetDelete(context.Context, string) (string, error)
// 	Flush(context.Context) error
// 	Close()
// }

// RedisStore represents redis storage
type RedisStore struct {
	redisClient *redis.Client
	redisLock   *redislock.Client
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
	rdb.redisLock = redislock.New(rdb.redisClient)

	// make sure connection is up
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	pong, err := rdb.redisClient.Ping(ctx).Result()
	if err != nil && strings.ToLower(pong) == "pong" {
		return RedisStore{}, nil
	}
	return rdb, nil
}

// // Get returns a string of the value stored
// func (rs RedisStore) Get(ctx context.Context, key string) (string, error) {
// 	return rs.redisClient.Get(ctx, key).Result()
// }

// // Set returns set the value of the key to value
// func (rs RedisStore) Set(ctx context.Context, key string, value string) error {
// 	return rs.redisClient.Set(ctx, key, value, 0).Err()
// }

// // SetIfNotExist sets the value if the key value doesn't exist in a atomic fashion
// func (rs RedisStore) SetIfNotExist(ctx context.Context, key string, value string) (bool, error) {
// 	return rs.redisClient.SetNX(ctx, key, value, 0).Result()
// }

// // GetDelete gets and deletes values atomically
// func (rs RedisStore) GetDelete(ctx context.Context, key string) (string, error) {
// 	pipe := rs.redisClient.TxPipeline()
// 	val, _ := pipe.Get(ctx, key).Result()
// 	pipe.Del(ctx, key).Result()
// 	_, err := pipe.Exec(ctx)
// 	return val, err
// }

// // Flush gets rid of all the keys in DB
// func (rs RedisStore) Flush(ctx context.Context) error {
// 	return rs.redisClient.FlushDB(ctx).Err()
// }

// // HSet sets keys in a hash
// func (rs RedisStore) HSet(ctx context.Context, key string, values ...interface{}) (int64, error) {
// 	return rs.redisClient.HSet(ctx, key, values).Result()
// }

// // HSetNX sets key in hash if it doesn't exist
// func (rs RedisStore) HSetNX(ctx context.Context, key string, field string, values ...interface{}) (bool, error) {
// 	return rs.redisClient.HSetNX(ctx, key, field, values).Result()
// }

// // HGet gets keys in a hash
// func (rs RedisStore) HGet(ctx context.Context, key string, field string) (string, error) {
// 	return rs.redisClient.HGet(ctx, key, field).Result()
// }

// // Close the store
// func (rs RedisStore) Close() error {
// 	return rs.redisClient.Close()
// }
