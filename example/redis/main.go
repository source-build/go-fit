package main

import (
	"github.com/source-build/go-fit"
	"log"
)

func main() {
	//连接redis单节点
	err := fit.NewDefaultRedisClient("127.0.0.1:6379", "", "", 0)
	if err != nil {
		log.Fatalln(err)
	}
	defer fit.CloseRedis()

	////连接redis单节点，自定义配置
	//err = fit.NewRedisClient(redis.Options{
	//	Addr:               "",
	//	Username:           "",
	//	Password:           "",
	//	DB:                 0,
	//	MinIdleConns:       0,
	//	MaxConnAge:         0,
	//	PoolTimeout:        0,
	//	IdleTimeout:        0,
	//	IdleCheckFrequency: 0,
	//	TLSConfig:          nil,
	//	Limiter:            nil,
	//})
	//if err != nil {
	//	log.Fatalln(err)
	//}
	//defer fit.CloseRedis()
	//
	////连接redis集群，默认0db
	//err = fit.NewDefaultRedisCluster([]string{"127.0.0.1:6379", "127.0.0.1:6379"}, "", "")
	//
	////连接redis集群，自定义配置
	//err = fit.NewRedisCluster(redis.ClusterOptions{
	//	Addrs:              nil,
	//	NewClient:          nil,
	//	MaxRedirects:       0,
	//	ReadOnly:           false,
	//	RouteByLatency:     false,
	//	RouteRandomly:      false,
	//	ClusterSlots:       nil,
	//	Dialer:             nil,
	//	OnConnect:          nil,
	//	Username:           "",
	//	Password:           "",
	//	MaxRetries:         0,
	//	MinRetryBackoff:    0,
	//	MaxRetryBackoff:    0,
	//	DialTimeout:        0,
	//	ReadTimeout:        0,
	//	WriteTimeout:       0,
	//	PoolFIFO:           false,
	//	PoolSize:           0,
	//	MinIdleConns:       0,
	//	MaxConnAge:         0,
	//	PoolTimeout:        0,
	//	IdleTimeout:        0,
	//	IdleCheckFrequency: 0,
	//	TLSConfig:          nil,
	//})

	/**
	 * 连接redis方式任意选一种就行，否则优先使用单节点
	 */

	/**
	  参数：可选
	  fit.CtxTimeout() 设置超时时间，默认10s
	  fit.DisableTimeout() 禁用超时时间
		fit.WithCtx() 传递context，不传 默认使用context.Background()
	  fit.WithGinTraceCtx() 传递gin.context,用于日志收集
		fit.WithExpire() 设置key过期时间，默认不过期
	*/
	instance := fit.RedisClient()
	//添加hook,GetClient() 获取单节点实例，GetCluster() 获取集群实例，取决于你初始化时用单节点连接还是集群连接
	//instance.GetCluster().AddHook()
	_, err = instance.Set("key", "value")
	if err != nil {
		log.Fatalln(err)
	}

}
