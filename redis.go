package fit

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"time"
)

const (
	Client = iota
	Cluster
)

var rClient *redis.Client
var rClusterClient *redis.ClusterClient

type RedisClientHook struct {
}

func (r RedisClientHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, "startTime", time.Now()), nil
}

func (r RedisClientHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	traceCtx := ctx.Value(GetTraceCtxName())
	if traceCtx == nil {
		return nil
	}

	trace, ok := ToTrace(traceCtx)
	if !ok {
		return nil
	}

	st, ok := ctx.Value("startTime").(time.Time)
	if !ok {
		return nil
	}

	trace.AppendRedis(&LinkTraceRedis{
		Timestamp: GetTimeStr(st),
		Handle:    cmd.Name(),
		Args:      cmd.Args(),
		Cost:      time.Since(st).String(),
	})
	return nil
}

func (r RedisClientHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (r RedisClientHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	return nil
}

func notFindInstance() (string, error) {
	return "", errors.New("not find redis instance")
}

// RedisOption Only common methods are provided here.
// You can use the Client or ClusterClient to extend the remaining methods
// Timeout：redis client timeout
// Incoming expire expires after a given time
type RedisOption struct {
	timeout time.Duration
	expire  time.Duration
	ctx     context.Context
}

type RedisOptionFunc func(*RedisOption)

func RedisClient(opts ...RedisOptionFunc) *RedisOption {
	option := &RedisOption{}
	option.timeout = time.Second * 10
	for _, opt := range opts {
		opt(option)
	}
	return option
}

func NewDefaultRedisClient(addr, username, password string, db int) error {
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

	rdb.AddHook(new(RedisClientHook))
	rClient = rdb
	return nil
}

func NewRedisClient(config redis.Options) error {
	rdb := redis.NewClient(&config)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return err
	}

	rdb.AddHook(new(RedisClientHook))
	rClient = rdb
	return nil
}

func NewDefaultRedisCluster(addr []string, username, password string) error {
	db := redis.NewClusterClient(&redis.ClusterOptions{
		//-------------------------------------------------------------------------------------------
		//集群相关的参数
		//集群节点地址，理论上只要填一个可用的节点客户端就可以自动获取到集群的所有节点信息。但是最好多填一些节点以增加容灾能力，因为只填一个节点的话，如果这个节点出现了异常情况，则Go应用程序在启动过程中无法获取到集群信息。
		Addrs: addr,

		MaxRedirects: 1, // 当遇到网络错误或者MOVED/ASK重定向命令时，最多重试几次，默认8

		//只含读操作的命令的"节点选择策略"。默认都是false，即只能在主节点上执行。
		ReadOnly: false, // 置为true则允许在从节点上执行只含读操作的命令
		// 默认false。 置为true则ReadOnly自动置为true,表示在处理只读命令时，可以在一个slot对应的主节点和所有从节点中选取Ping()的响应时长最短的一个节点来读数据
		RouteByLatency: false,
		// 默认false。置为true则ReadOnly自动置为true,表示在处理只读命令时，可以在一个slot对应的主节点和所有从节点中随机挑选一个节点来读数据
		RouteRandomly: false,

		//用户可定制读取节点信息的函数，比如在非集群模式下可以从zookeeper读取。
		//但如果面向的是redis cluster集群，则客户端自动通过cluster slots命令从集群获取节点信息，不会用到这个函数。
		//ClusterSlots: func(context.Context){},

		//钩子函数，当一个新节点创建时调用，传入的参数是新建的redis.Client
		//OnNewNode: func(*Client) {},

		//------------------------------------------------------------------------------------------------------
		//ClusterClient管理着一组redis.Client,下面的参数和非集群模式下的redis.Options参数一致，但默认值有差别。
		//初始化时，ClusterClient会把下列参数传递给每一个redis.Client

		//钩子函数
		//仅当客户端执行命令需要从连接池获取连接时，如果连接池需要新建连接则会调用此钩子函数
		//OnConnect: func(ctx context.Context, conn *redis.Conn) error {
		//	fmt.Printf("钩子函数conn=%v\n", conn)
		//	return nil
		//},

		Password: password,
		Username: username,

		//每一个redis.Client的连接池容量及闲置连接数量，而不是cluterClient总体的连接池大小。实际上没有总的连接池
		//而是由各个redis.Client自行去实现和维护各自的连接池。
		PoolSize:     15, // 连接池最大socket连接数，默认为5倍CPU数， 5 * runtime.NumCPU
		MinIdleConns: 10, //在启动阶段创建指定数量的Idle连接，并长期维持idle状态的连接数不少于指定数量；。

		//命令执行失败时的重试策略
		MaxRetries:      2,                      // 命令执行失败时，最多重试多少次，默认为0即不重试
		MinRetryBackoff: 8 * time.Millisecond,   //每次计算重试间隔时间的下限，默认8毫秒，-1表示取消间隔
		MaxRetryBackoff: 512 * time.Millisecond, //每次计算重试间隔时间的上限，默认512毫秒，-1表示取消间隔

		//超时
		DialTimeout:  5 * time.Second, //连接建立超时时间，默认5秒。
		ReadTimeout:  3 * time.Second, //读超时，默认3秒， -1表示取消读超时
		WriteTimeout: 3 * time.Second, //写超时，默认等于读超时，-1表示取消读超时
		PoolTimeout:  4 * time.Second, //当所有连接都处在繁忙状态时，客户端等待可用连接的最大等待时长，默认为读超时+1秒。

		//闲置连接检查包括IdleTimeout，MaxConnAge
		IdleCheckFrequency: 60 * time.Second, //闲置连接检查的周期，无默认值，由ClusterClient统一对所管理的redis.Client进行闲置连接检查。初始化时传递-1给redis.Client表示redis.Client自己不用做周期性检查，只在客户端获取连接时对闲置连接进行处理。
		IdleTimeout:        5 * time.Minute,  //闲置超时，默认5分钟，-1表示取消闲置超时检查
		MaxConnAge:         0 * time.Second,  //连接存活时长，从创建开始计时，超过指定时长则关闭连接，默认为0，即不关闭存活时长较长的连接
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_, err := db.Ping(ctx).Result()
	if err != nil {
		return err
	}
	db.AddHook(new(RedisClientHook))
	rClusterClient = db
	return nil
}

func NewRedisCluster(config redis.ClusterOptions) error {
	db := redis.NewClusterClient(&config)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_, err := db.Ping(ctx).Result()
	if err != nil {
		return err
	}
	db.AddHook(new(RedisClientHook))
	rClusterClient = db
	return nil
}

func CloseRedis() {
	if rClient != nil {
		_ = rClient.Close()
	}
	if rClusterClient != nil {
		_ = rClusterClient.Close()
	}
}

func testClient() int {
	if rClient != nil {
		return Client
	}
	if rClusterClient != nil {
		return Cluster
	}
	return -1
}

// DisableTimeout set operation timeout
func DisableTimeout() RedisOptionFunc {
	return func(c *RedisOption) {
		c.timeout = 0
	}
}

// WithExpire set key timeout
func WithExpire(t time.Duration) RedisOptionFunc {
	return func(c *RedisOption) {
		c.expire = t
	}
}

// CtxTimeout set operation timeout
func CtxTimeout(t time.Duration) RedisOptionFunc {
	return func(c *RedisOption) {
		c.timeout = t
	}
}

// WithCtx set operation timeout
func WithCtx(ctx context.Context) RedisOptionFunc {
	return func(c *RedisOption) {
		c.ctx = ctx
	}
}

func WithGinTraceCtx(g *gin.Context) RedisOptionFunc {
	return func(c *RedisOption) {
		trace, ok := GetGinTraceCtx(g)
		ctx := context.Background()
		if !ok {
			c.ctx = ctx
			return
		}
		c.ctx = context.WithValue(ctx, GetTraceCtxName(), trace)
	}
}

func (r *RedisOption) GetClient() *redis.Client {
	return rClient
}

func (r *RedisOption) GetCluster() *redis.ClusterClient {
	return rClusterClient
}

func (r *RedisOption) Set(key string, value interface{}) (string, error) {
	if r.ctx == nil {
		r.ctx = context.Background()
	}
	ctx := r.ctx
	if r.timeout > 0 {
		ctx2, cancel := context.WithTimeout(r.ctx, r.timeout)
		defer cancel()
		ctx = ctx2
	}
	if testClient() == Client {
		return rClient.Set(ctx, key, value, r.expire).Result()
	}
	if testClient() == Cluster {
		return rClusterClient.Set(ctx, key, value, r.expire).Result()
	}
	return notFindInstance()
}

func (r *RedisOption) Get(key string) (string, error) {
	if r.ctx == nil {
		r.ctx = context.Background()
	}
	ctx := r.ctx
	if r.timeout > 0 {
		ctx2, cancel := context.WithTimeout(r.ctx, r.timeout)
		defer cancel()
		ctx = ctx2
	}
	if testClient() == Client {
		return rClient.Get(ctx, key).Result()
	}
	if testClient() == Cluster {
		return rClusterClient.Get(ctx, key).Result()
	}
	return notFindInstance()
}

// HSet Set the value of the field  in the hash table key to value
func (r *RedisOption) HSet(key string, values ...interface{}) (bool, error) {
	if r.ctx == nil {
		r.ctx = context.Background()
	}
	ctx := r.ctx
	if r.timeout > 0 {
		ctx2, cancel := context.WithTimeout(r.ctx, r.timeout)
		defer cancel()
		ctx = ctx2
	}
	var ok bool
	var err error
	if testClient() == Client {
		ok, err = rClient.HMSet(ctx, key, values).Result()
		if r.expire > 0 {
			ok, err = rClient.Expire(ctx, key, r.expire).Result()
		}
	}
	if testClient() == Cluster {
		ok, err = rClusterClient.HMSet(ctx, key, values).Result()
		if r.expire > 0 {
			ok, err = rClusterClient.Expire(ctx, key, r.expire).Result()
		}
	}
	return ok, err
}

// HGet Get values for all given fields
func (r *RedisOption) HGet(key, field string) (string, error) {
	if r.ctx == nil {
		r.ctx = context.Background()
	}
	ctx := r.ctx
	if r.timeout > 0 {
		ctx2, cancel := context.WithTimeout(r.ctx, r.timeout)
		defer cancel()
		ctx = ctx2
	}
	if testClient() == Client {
		return rClient.HGet(ctx, key, field).Result()
	}
	if testClient() == Cluster {
		return rClusterClient.HGet(ctx, key, field).Result()
	}
	return notFindInstance()
}

// HGetAll HMGet Get all fields and values of the specified key in the hash table
func (r *RedisOption) HGetAll(key string) (map[string]string, error) {
	if r.ctx == nil {
		r.ctx = context.Background()
	}
	ctx := r.ctx
	if r.timeout > 0 {
		ctx2, cancel := context.WithTimeout(r.ctx, r.timeout)
		defer cancel()
		ctx = ctx2
	}
	if testClient() == Client {
		return rClient.HGetAll(ctx, key).Result()
	}
	if testClient() == Cluster {
		return rClusterClient.HGetAll(ctx, key).Result()
	}
	return nil, errors.New("not find client")
}

// HMGet Get values for all given fields
// example: HMGET key field1 [field2] [field3] ...
func (r *RedisOption) HMGet(key string, fields ...string) ([]interface{}, error) {
	if r.ctx == nil {
		r.ctx = context.Background()
	}
	ctx := r.ctx
	if r.timeout > 0 {
		ctx2, cancel := context.WithTimeout(r.ctx, r.timeout)
		defer cancel()
		ctx = ctx2
	}
	if testClient() == Client {
		return rClient.HMGet(ctx, key, fields...).Result()
	}
	if testClient() == Cluster {
		return rClusterClient.HMGet(ctx, key, fields...).Result()
	}
	return nil, errors.New("not find client")
}

// HLen  Gets the number of fields in the hash table
func (r *RedisOption) HLen(key string) (int64, error) {
	if r.ctx == nil {
		r.ctx = context.Background()
	}
	ctx := r.ctx
	if r.timeout > 0 {
		ctx2, cancel := context.WithTimeout(r.ctx, r.timeout)
		defer cancel()
		ctx = ctx2
	}
	if testClient() == Client {
		return rClient.HLen(ctx, key).Result()
	}
	if testClient() == Cluster {
		return rClusterClient.HLen(ctx, key).Result()
	}
	return 0, errors.New("not find client")
}
