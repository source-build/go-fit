package fit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisMode represents the Redis deployment mode
type RedisMode int

const (
	// RedisModeSingle represents single-node Redis deployment
	RedisModeSingle RedisMode = iota
	// RedisModeCluster represents Redis cluster deployment
	RedisModeCluster
)

// RedisClient provides a unified Redis client that supports both single-node and cluster modes.
// It embeds redis.Cmdable interface to automatically inherit all Redis command methods,
// eliminating the need for manual method delegation and ensuring compatibility with
// future redis library updates.
type RedisClient struct {
	redis.Cmdable // Embedded interface providing all Redis command methods
	client        *redis.Client
	clusterClient *redis.ClusterClient
	mode          RedisMode
}

// RDB is the global Redis client instance that provides unified access
// to Redis operations regardless of the underlying deployment mode
var RDB *RedisClient

// RedisConfig defines the configuration options for Redis client initialization.
// It supports both single-node and cluster configurations through the Mode field.
type RedisConfig struct {
	Mode     RedisMode `json:"mode" yaml:"mode"`         // Redis deployment mode
	Addr     string    `json:"addr" yaml:"addr"`         // Single-node address
	Addrs    []string  `json:"addrs" yaml:"addrs"`       // Cluster node addresses
	Username string    `json:"username" yaml:"username"` // Authentication username
	Password string    `json:"password" yaml:"password"` // Authentication password
	DB       int       `json:"db" yaml:"db"`             // Database number (single-node only)
}

// NewRedis creates and initializes a unified Redis client based on the provided configuration.
// It automatically selects between single-node and cluster mode based on config.Mode.
// The initialized client is stored in the global RDB variable for application-wide access.
//
// Example:
//
//	// Single-node configuration
//	err := NewRedis(RedisConfig{
//	    Mode: RedisModeSingle,
//	    Addr: "localhost:6379",
//	    DB:   0,
//	})
//
//	// Cluster configuration
//	err := NewRedis(RedisConfig{
//	    Mode: RedisModeCluster,
//	    Addrs: []string{"redis1:6379", "redis2:6379", "redis3:6379"},
//	})
func NewRedis(config RedisConfig) error {
	switch config.Mode {
	case RedisModeCluster:
		return newRedisCluster(config)
	default:
		return newRedisSingle(config)
	}
}

// newRedisSingle creates a single-node Redis client with the provided configuration.
// It performs connection validation and stores the client in the global RDB variable.
func newRedisSingle(config RedisConfig) error {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Username: config.Username,
		Password: config.Password,
		DB:       config.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return err
	}

	RDB = &RedisClient{
		Cmdable: client,
		client:  client,
		mode:    RedisModeSingle,
	}

	return nil
}

// newRedisCluster creates a Redis cluster client with the provided configuration.
// It validates connectivity to all cluster nodes and stores the client in the global RDB variable.
func newRedisCluster(config RedisConfig) error {
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    config.Addrs,
		Username: config.Username,
		Password: config.Password,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	err := client.ForEachShard(ctx, func(ctx context.Context, shard *redis.Client) error {
		return shard.Ping(ctx).Err()
	})
	if err != nil {
		return err
	}

	RDB = &RedisClient{
		Cmdable:       client,
		clusterClient: client,
		mode:          RedisModeCluster,
	}

	return nil
}

// GetMode returns the current Redis deployment mode (single-node or cluster).
func (r *RedisClient) GetMode() RedisMode {
	return r.mode
}

// GetRawClient returns the underlying Redis client for advanced operations
// that may not be available through the unified interface.
// Returns *redis.Client for single-node mode or *redis.ClusterClient for cluster mode.
func (r *RedisClient) GetRawClient() interface{} {
	if r.mode == RedisModeCluster {
		return r.clusterClient
	}
	return r.client
}

// Close gracefully closes the Redis connection and releases associated resources.
// It handles both single-node and cluster client cleanup automatically.
func (r *RedisClient) Close() error {
	if r.mode == RedisModeCluster && r.clusterClient != nil {
		return r.clusterClient.Close()
	}
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// NewRedisDefaultClient creates a single-node Redis client using simplified parameters.
// This is a convenience function for quick setup with common configuration options.
//
// Parameters:
//   - addr: Redis server address (e.g., "localhost:6379")
//   - username: Authentication username (empty string if not required)
//   - password: Authentication password (empty string if not required)
//   - db: Database number to select
func NewRedisDefaultClient(addr, username, password string, db int) error {
	return NewRedis(RedisConfig{
		Mode:     RedisModeSingle,
		Addr:     addr,
		Username: username,
		Password: password,
		DB:       db,
	})
}

// NewRedisClient creates a single-node Redis client using the official redis.Options configuration.
// This function provides full control over Redis client configuration options.
func NewRedisClient(config redis.Options) error {
	client := redis.NewClient(&config)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return err
	}

	RDB = &RedisClient{
		Cmdable: client,
		client:  client,
		mode:    RedisModeSingle,
	}

	return nil
}

// NewRedisDefaultClusterClient creates a Redis cluster client using simplified parameters.
// This is a convenience function for quick cluster setup with common configuration options.
//
// Parameters:
//   - addrs: List of cluster node addresses
//   - username: Authentication username (empty string if not required)
//   - password: Authentication password (empty string if not required)
func NewRedisDefaultClusterClient(addrs []string, username, password string) error {
	return NewRedis(RedisConfig{
		Mode:     RedisModeCluster,
		Addrs:    addrs,
		Username: username,
		Password: password,
	})
}

// NewRedisClusterClient creates a Redis cluster client using the official redis.ClusterOptions configuration.
// This function provides full control over Redis cluster client configuration options.
func NewRedisClusterClient(config redis.ClusterOptions) error {
	client := redis.NewClusterClient(&config)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	err := client.ForEachShard(ctx, func(ctx context.Context, shard *redis.Client) error {
		return shard.Ping(ctx).Err()
	})
	if err != nil {
		return err
	}

	RDB = &RedisClient{
		Cmdable:       client,
		clusterClient: client,
		mode:          RedisModeCluster,
	}

	return nil
}

// CloseRedis closes the global Redis connection and releases associated resources.
// This function should be called during application shutdown to ensure proper cleanup.
func CloseRedis() {
	if RDB != nil {
		_ = RDB.Close()
	}
}

// GetRedisMode returns the current Redis deployment mode of the global client.
// Returns RedisModeSingle if no client is initialized.
func GetRedisMode() RedisMode {
	if RDB != nil {
		return RDB.GetMode()
	}
	return RedisModeSingle
}

// GetRawRedisClient returns the underlying Redis client instance for advanced operations.
// Returns nil if no client is initialized.
// The returned type will be *redis.Client for single-node or *redis.ClusterClient for cluster mode.
func GetRawRedisClient() interface{} {
	if RDB != nil {
		return RDB.GetRawClient()
	}
	return nil
}
