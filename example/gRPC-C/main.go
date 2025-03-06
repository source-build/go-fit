package main

import (
	"context"
	"fmt"
	"github.com/source-build/go-fit/frpc"
	"github.com/source-build/go-fit/pb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
)

// Authentication Token认证方式
type Authentication struct {
	User     string
	Password string
}

// GetRequestMetadata 返回需要认证的必要信息
func (a *Authentication) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"user": a.User, "password": a.Password}, nil
}

// RequireTransportSecurity 是否使用安全链接(TLS)
func (a *Authentication) RequireTransportSecurity() bool {
	return false
}

func direct() {
	var opts []grpc.DialOption
	// 禁用传输安全
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	// Token验证
	//user := &Authentication{
	//	User:     "foo1",
	//	Password: "admin2",
	//}
	//opts = append(opts, grpc.WithPerRPCCredentials(user))

	// 直连方式，直接调用的是 grpc.DialContext
	conn, err := frpc.NewDirectClient("127.0.0.1:8888", opts...)
	if err != nil {
		log.Fatal(err)
	}

	c := pb.NewPhoneLoginSmsVerCodeClient(conn)
	resp, err := c.Check(context.Background(), &pb.CheckRequest{
		PhoneCode: "123456",
		Code:      0,
	})
	fmt.Println(resp, err)
}

func gRPCDirect() {
	clientV3, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"110.42.184.124:2479"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// 调用此方法进行初始化以完成注册
	err = frpc.Init(frpc.RpcClientConf{
		// etcd
		EtcdClient: clientV3,
		// 命名空间
		Namespace: "ha",

		// TLS单向认证，只有客户端验证服务器的身份
		//TLSType: frpc.TLSTypeOneWay,
		//// 公钥证书文件路径
		//CertFile: "example/k/server.pem",
		//// 域名
		//ServerNameOverride: "www.sourcebuild.cn",

		// TLS双向认证，客户端不仅验证服务器的证书，服务器也验证客户端的证书。
		TLSType:            frpc.TLSTypeMTLS,
		CertFile:           "keys/client.crt",
		KeyFile:            "keys/client.key",
		CAFile:             "keys/ca.crt",
		ServerNameOverride: "ht.sourcebuild.cn",
	})
	if err != nil {
		log.Fatal(err)
	}

	// 参数选项
	var opts []frpc.DialOption
	// 接收 grpc.DialOption
	opts = append(opts, frpc.WithGrpcOption(grpc.WithTransportCredentials(insecure.NewCredentials())))

	// 负载均衡器
	// 选择第一个健康的客户端(gRPC默认负载均衡策略)
	opts = append(opts, frpc.WithGrpcOption(grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"pick_first"}`)))
	// 随机
	opts = append(opts, frpc.WithBalancerRandom())
	// 轮询
	opts = append(opts, frpc.WithBalancerRoundRobin())
	// 加权轮询
	opts = append(opts, frpc.WithBalancerWeightRoundRobin())

	// 阻塞连接，直到建立连接成功。如果不设置，NewClient 会立即返回 Dial，并在后台异步进行连接服务器的过程。
	// 这个方法可以确保客户端在发送RPC请求之前已经建立了连接。
	//
	// 这个方法会一直阻塞，可以调用 frpc.WithCtx() 传入 Context 手动取消连接，
	// 也可以调用 frpc.WithTimeoutCtx() 传入一个超时时间，该 Context 仅在设置 frpc.WithBlock() 时有效，用于连接超时处理。
	//opts = append(opts, frpc.WithBlock())

	// 禁用TLS身份验证，如果在frpc.Init使用了TLS(单/双向认证),但是又希望在本次连接中不使用身份验证，可使用该方法来禁用它。
	//opts = append(opts, frpc.DisableTLS())

	// 参数target: key(即服务注册时填写的key)
	client, err := frpc.NewClient("user", opts...)
	if err != nil {
		log.Println(err)
		return
	}
	defer client.Close()

	// 判断错误是否是“没有可用的服务”，该错误表示没有找到任何服务。
	//frpc.IsNotFoundServiceErr(err)

	id, err := pb.NewUserInfoClient(client).GetUserInfoById(context.Background(), &pb.UserInfoIdRequest{Id: 1})
	fmt.Println(id, err)
}

func main() {
	//direct()

	gRPCDirect()
}
