package fit

import (
	"context"
	"github.com/go-redis/redis/v8"
	"time"
)

const (
	Client = iota
	Cluster
)

var RDB *redis.Client

var RCDB *redis.ClusterClient

// NewRedisDefaultClient Create Redis client using shortcuts
func NewRedisDefaultClient(addr, username, password string, db int) error {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Username: username,
		Password: password,
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return err
	}

	RDB = rdb

	return nil
}

// NewRedisClient Create Redis client
func NewRedisClient(config redis.Options) error {
	rdb := redis.NewClient(&config)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return err
	}

	RDB = rdb

	return nil
}

// NewRedisDefaultClusterClient Create Redis cluster client using shortcuts
func NewRedisDefaultClusterClient(addr []string, username, password string) error {
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    addr,
		Password: password,
		Username: username,
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	err := rdb.ForEachShard(ctx, func(ctx context.Context, shard *redis.Client) error {
		return shard.Ping(ctx).Err()
	})
	if err != nil {
		return err
	}

	RCDB = rdb

	return nil
}

// NewRedisClusterClient Create Redis cluster client
func NewRedisClusterClient(config redis.ClusterOptions) error {
	rdb := redis.NewClusterClient(&config)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	err := rdb.ForEachShard(ctx, func(ctx context.Context, shard *redis.Client) error {
		return shard.Ping(ctx).Err()
	})
	if err != nil {
		return err
	}

	RCDB = rdb

	return nil
}

func CloseRedis() {
	if RDB != nil {
		_ = RDB.Close()
	}

	if RCDB != nil {
		_ = RCDB.Close()
	}
}
