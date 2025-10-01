package main

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/source-build/go-fit"
)

// 单机版
// 快速连接到redis服务器
func standAlone() {
	addr := "127.0.0.1:6379"
	username := ""
	password := ""
	db := 0

	err := fit.NewRedisDefaultClient(addr, username, password, db)
	if err != nil {
		log.Fatal(err)
	}

	defer fit.CloseRedis()

	fit.RDB.Get(context.Background(), "foo").Result()
}

// 单机版
// 自定义配置连接到redis服务器
func standAloneConfig() {
	opt := redis.Options{Addr: "127.0.0.1:6379"}
	err := fit.NewRedisClient(opt)
	if err != nil {
		log.Fatal(err)
	}

	defer fit.CloseRedis()

	fit.RDB.Get(context.Background(), "").Result()
}

// 集群
// 快速连接到redis服务器
func cluster() {
	addrs := []string{"127.0.0.1:7000", "127.0.0.1:7001", "127.0.0.1:7002", "127.0.0.1:7006", "127.0.0.1:7004", "127.0.0.1:7005"}
	username := ""
	password := ""
	err := fit.NewRedisDefaultClusterClient(addrs, username, password)
	if err != nil {
		log.Fatal(err)
	}
	defer fit.CloseRedis()

	fit.RDB.Get(context.Background(), "foo").Result()
}

// 集群
// 自定义配置连接到redis服务器
func clusterConfig() {
	addrs := []string{"127.0.0.1:7000", "127.0.0.1:7001", "127.0.0.1:7002", "127.0.0.1:7006", "127.0.0.1:7004", "127.0.0.1:7005"}

	opt := redis.ClusterOptions{Addrs: addrs}
	err := fit.NewRedisClusterClient(opt)
	if err != nil {
		log.Fatal(err)
	}

	defer fit.CloseRedis()

	fit.RDB.Get(context.Background(), "foo").Result()
}

func main() {
	// 单机版
	// 快速连接到redis服务器
	standAlone()
	// 自定义配置连接到redis服务器
	//standAloneConfig()

	// 使用 fit.RDB 访问 *redis.Client

	// 集群
	// 快速连接到redis服务器
	//cluster()

	// 自定义配置连接到redis服务器
	//clusterConfig()
}
