package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/source-build/go-fit/frpc"
	"github.com/source-build/go-fit/pb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

// Authentication Token验证
type Authentication struct {
	User     string
	Password string
}

// GetRequestMetadata 返回需要认证的必要信息
func (a *Authentication) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	fmt.Println("GetRequestMetadata", a.User, a.Password)
	return map[string]string{"user": a.User, "password": a.Password}, nil
}

// RequireTransportSecurity 是否使用安全链接(TLS)
func (a *Authentication) RequireTransportSecurity() bool {
	return false
}

// 直连方式
func direct() {
	var opts []grpc.DialOption
	// 禁用传输安全
	// opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	// 自定义配置...

	conn, err := frpc.NewDirectClient("192.168.1.5:8888", opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	c := pb.NewPhoneLoginSmsVerCodeClient(conn)
	resp, err := c.Check(context.Background(), &pb.CheckRequest{
		PhoneCode: "123456",
		Code:      0,
	})
	fmt.Println("resp:", resp, err)
}

// 服务发现模式
func serviceDiscovery() {
	clientV3, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// 初始化客户端(必须)
	err = frpc.Init(frpc.RpcClientConf{
		// etcd 客户端
		EtcdClient: clientV3,
		// 命名空间
		Namespace: "ht",

		// 连接池配置
		PoolConfig: &frpc.PoolConfig{
			// 最大连接(gRPC连接)空闲时间，默认 30 分钟
			MaxIdleTime: 30 * time.Minute,
			// 连接(gRPC连接)清理检查间隔，默认 5 分钟
			CleanupTicker: 5 * time.Minute,
			// 并发阈值，默认 500
			// 当最小连接数的连接并发数超过此阈值时，会创建新的连接
			ConcurrencyThreshold: 1000,

			// 最大的服务连接(gRPC连接)的数量，默认 5
			// 创建的服务连接(gRPC连接)数量超过此阈值时，不再创建新的服务连接，而是从现有服务中获取连接数最少的服务(gRPC连接)使用
			MaxConnectionsPerID: 10,
			// 每个服务连接实例的最小连接数，默认 1（每个服务连接实例至少保持 1 个连接）
			MinConnectionsPerID: 1,
		},

		// ==================== Token认证(可选，下方的TLS必须配置) ====================
		// Token认证凭据，
		//TokenCredentials: &Authentication{
		//	User:     "foo1",
		//	Password: "admin",
		//},

		// ==================== TLS单向认证(必选，与 双向认证 二选一) ====================
		// 只有客户端验证服务器的身份
		//TransportType: frpc.TransportTypeOneWay,
		//// 公钥证书文件路径
		//CertFile: "example/k/server.pem",
		//// 域名
		//ServerNameOverride: "www.sourcebuild.cn",

		// ==================== TLS双向认证(必选，与 单向认证 二选一) ====================
		// 客户端不仅验证服务器的证书，服务器也验证客户端的证书
		TransportType:      frpc.TransportTypeMTLS,
		CertFile:           "keys/client.crt",
		KeyFile:            "keys/client.key",
		CAFile:             "keys/ca.crt",
		ServerNameOverride: "www.sourcebuild.cn",
	})
	if err != nil {
		panic(err)
	}
	defer frpc.ClosePool() // 关闭连接池

	fmt.Println("🎯 初始化完成！按回车键开始测试，输入'q'退出...")

	for {
		fmt.Print("请按回车开始测试 (输入'q'退出): ")
		var input string
		fmt.Scanln(&input)
		if input == "q" || input == "Q" {
			fmt.Println("👋 退出程序")
			break
		}

		go runConcurrentTest()
	}
}

// runConcurrentTest 执行并发测试
func runConcurrentTest() {
	// 并发数量
	const concurrency = 10000
	// 失败次数
	errCount := int64(0)

	var wg sync.WaitGroup

	fmt.Printf("🚀 启动 %d 个并发请求...\n", concurrency)
	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := makeRequest(id)
			if err != nil {
				atomic.AddInt64(&errCount, 1)
				return
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	fmt.Printf("✅ %d 个并发请求完成，耗时: %v\n", concurrency, duration)
	fmt.Printf("   平均每个请求: %v\n", duration/time.Duration(concurrency))
	cc := atomic.LoadInt64(&errCount)
	fmt.Printf("📊 模拟请求量: %v | 请求成功数量: %v | 请求失败数量: %v\n\n", concurrency, int64(concurrency)-cc, cc)
}

// makeRequest 发起单个请求
func makeRequest(id int) error {
	// 参数选项
	var opts []frpc.DialOptions
	// 接收 grpc.DialOption
	//opts = append(opts, frpc.WithGrpcOption(...))

	// 负载均衡器
	// 选择第一个健康的客户端(gRPC默认负载均衡策略，即没有负载均衡效果)
	//opts = append(opts, frpc.WithGrpcOption(grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"pick_first"}`)))
	// 随机
	//opts = append(opts, frpc.WithBalancerRandom())
	// 轮询(frpc默认)
	opts = append(opts, frpc.WithBalancerRoundRobin())
	// 最少连接数，请求处理时间差异较大的服务，选择当前活跃连接数最少的服务实例
	//opts = append(opts, frpc.WithBalancerLeastConn())
	// 加权轮询
	//opts = append(opts, frpc.WithBalancerWeightRoundRobin())

	// target: 服务注册时的key
	client, err := frpc.NewClient("user", opts...)
	if err != nil {
		log.Printf("请求%d: 获取连接失败 %v", id, err)
		return err
	}
	defer client.Close()

	_, err = pb.NewPhoneLoginSmsVerCodeClient(client).Send(context.Background(), &pb.SendRequest{
		PhoneCode: "123456",
	})
	if err != nil {
		state := client.GetState()
		log.Println("请求失败", state, err)
		return err
	}

	// 模拟请求处理时间
	//time.Sleep(500 * time.Millisecond)

	return nil
}

func main() {
	// 直连模式
	//direct()

	// 服务发现模式
	serviceDiscovery()
}
