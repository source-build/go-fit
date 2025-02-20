package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"github.com/source-build/go-fit"
	"github.com/source-build/go-fit/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var port string

type phoneSms struct {
	pb.UnimplementedPhoneLoginSmsVerCodeServer
}

func (p phoneSms) Send(ctx context.Context, request *pb.SendRequest) (*pb.Response, error) {
	return &pb.Response{
		Msg:    "OK",
		Code:   0,
		Result: "OK",
	}, nil
}

func (p phoneSms) Check(_ context.Context, request *pb.CheckRequest) (*pb.Response, error) {
	return &pb.Response{
		Msg:    "OK",
		Code:   0,
		Result: port,
	}, nil
}

// 身份验证拦截器
func authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	md, _ := metadata.FromIncomingContext(ctx)
	fmt.Println(md)
	var user string

	if val, ok := md["user"]; ok {
		user = val[0]
	}

	if user != "foo" {
		return
	}

	// 继续处理请求
	return handler(ctx, req)
}

var weight = flag.Int("w", 0, "weight")

func main() {
	flag.Parse()

	freePort, err := fit.GetFreePort()
	if err != nil {
		return
	}

	port = strconv.Itoa(freePort)

	listen, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalln(err)
	}

	var opts []grpc.ServerOption
	// Token验证
	//opts = append(opts, grpc.UnaryInterceptor(authInterceptor))

	// 单向认证

	//引入TLS认证相关文件(传入公钥和私钥)
	//配置 TLS认证相关文件
	//creds, err := credentials.NewServerTLSFromFile("example/k/server.pem", "example/k/server.key")

	//添加TLS认证
	//opts = append(opts, grpc.Creds(creds))

	// 双向认证
	opts = append(opts, newServerTls())

	rpcServer := grpc.NewServer(opts...)

	pb.RegisterPhoneLoginSmsVerCodeServer(rpcServer, new(phoneSms))

	reg, err := fit.NewRegisterService(fit.RegisterOptions{
		// 命名空间，默认使用 default
		Namespace: "ht",
		// 服务类型，可选 "api" 与 "rpc"，默认rpc
		ServiceType: "rpc",
		// 注册中心key，通常为服务名(如user)
		Key: "user",
		// 服务IP，填写 "*" 将自动获取网络出口ip。
		IP: "*",
		// 服务端口
		Port: port,
		// 服务离线时最大重试等待时间，不传则一直阻塞等待，直到etcd恢复
		MaxTimeoutRetryTime: time.Second * 9,
		// etcd 配置
		EtcdConfig: fit.EtcdConfig{
			Endpoints: []string{"127.0.0.1:2379"},
		},
		// zap 日志配置
		//Logger: flog.Logger(),
		// 租约时间
		TimeToLive: 10,
		// 自定义元数据
		Meta: fit.H{
			// 设置服务权重，权重越大，服务被调用的次数越多
			"weight": *weight,
		},
	})
	if err != nil {
		log.Fatal("服务注册失败")
	}

	defer reg.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		fmt.Println("服务启动成功!", listen.Addr().String())
		if err := rpcServer.Serve(listen); err != nil {
			log.Fatalln(err)
		}
	}()
	<-quit
	fmt.Println("service close")
}

func newServerTls() grpc.ServerOption {
	cert, err := tls.LoadX509KeyPair("keys/server.crt", "keys/server.key")
	if err != nil {
		log.Fatalln(err)
	}

	caCert, err := os.ReadFile("keys/ca.crt")
	if err != nil {
		log.Fatalln(err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		log.Fatalln("failed to append ca certs")
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
	}

	return grpc.Creds(credentials.NewTLS(config))
}
