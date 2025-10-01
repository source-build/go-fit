package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/source-build/go-fit"
	"github.com/source-build/go-fit/flog"
	"github.com/source-build/go-fit/pb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

var port string

type phoneSmsServer struct {
	pb.UnimplementedPhoneLoginSmsVerCodeServer
}

func NewPhoneSmsServer() pb.PhoneLoginSmsVerCodeServer {
	return &phoneSmsServer{}
}

func (p phoneSmsServer) Send(ctx context.Context, request *pb.SendRequest) (*pb.Response, error) {
	return &pb.Response{
		Msg:    "OK",
		Code:   0,
		Result: "OK",
	}, nil
}

func (p phoneSmsServer) Check(_ context.Context, request *pb.CheckRequest) (*pb.Response, error) {
	return &pb.Response{
		Msg:    "OK",
		Code:   0,
		Result: port,
	}, nil
}

// Token方式验证-身份验证拦截器
func authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	md, _ := metadata.FromIncomingContext(ctx)
	var user string

	if val, ok := md["user"]; ok {
		user = val[0]
	}

	if user != "foo" {
		return nil, fmt.Errorf("user %s not authorized", user)
	}

	// 继续处理请求
	return handler(ctx, req)
}

var weight = flag.Int("w", 0, "weight")

func main() {
	flag.Parse()
	// ==================== 初始化日志 ====================
	opt := flog.Options{
		LogLevel:          flog.InfoLevel,
		EncoderConfigType: flog.ProductionEncoderConfig,
		Console:           true,
		// 默认文件输出，为空表示不输出到文件
		Filename:   "logs/logger.log",
		MaxSize:    0,
		MaxAge:     0,
		MaxBackups: 0,
		LocalTime:  false,
		Compress:   false,
		Tees:       nil,
		ZapOptions: nil,
		CallerSkip: 0,
	}
	flog.Init(opt)
	defer flog.Sync()

	// 获取随机端口
	freePort, err := fit.GetFreePort()
	if err != nil {
		return
	}

	port = strconv.Itoa(freePort)
	listen, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalln(err)
	}

	// 注意：TLS 属于传输层凭据，必须配置，而 Per-RPC Credentials (Token) 是每次RPC凭据

	var opts []grpc.ServerOption
	// ==================== Token验证(可选，下方的TLS必须配置) ====================
	//opts = append(opts, grpc.UnaryInterceptor(authInterceptor))

	// ==================== TLS单向认证(必选，与 双向认证 二选一) ====================
	// 引入TLS认证相关文件(传入公钥和私钥)
	// 配置相关文件
	//creds, err := credentials.NewServerTLSFromFile("example/k/server.pem", "example/k/server.key")
	//opts = append(opts, grpc.Creds(creds))

	// ==================== TLS双向认证(必选，与 单向认证 二选一) ====================
	opts = append(opts, func() grpc.ServerOption {
		// 服务端证书
		cert, err := tls.LoadX509KeyPair("keys/server.crt", "keys/server.key")
		if err != nil {
			log.Fatalln(err)
		}

		// 根证书
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
	}())

	rpcServer := grpc.NewServer(opts...)

	// 注册grpc服务
	pb.RegisterPhoneLoginSmsVerCodeServer(rpcServer, NewPhoneSmsServer())

	// 服务注册
	reg, err := fit.NewRegisterService(fit.RegisterOptions{
		// 命名空间，默认使用 default，注册到etcd时的namespace
		Namespace: "ht",
		// 服务类型，支持注册 api 与 rpc 服务
		// 可选 "api" 与 "rpc"，默认rpc
		ServiceType: "rpc",
		// 注册中心中服务的key，通常为服务名(如user)
		Key: "user",
		// 服务ip，填写 "*" 自动获取网络出口ip(局域网)。
		IP: "*",
		// 服务端口
		Port: port,
		// 租约时间，单位秒，默认10秒
		TimeToLive: 10,
		// 服务断线最大超时重试次数，0表示无限次数(推荐)
		MaxRetryAttempts: 0,
		// etcd 配置
		EtcdConfig: clientv3.Config{
			Endpoints:   []string{"127.0.0.1:2379"},
			DialTimeout: time.Second * 5,
		},
		// zap 日志配置
		Logger: flog.ZapLogger(),
		// 自定义元数据
		Meta: fit.H{
			// 设置服务权重，权重越大，服务被调用的次数越多
			"weight": *weight,
		},
	})
	if err != nil {
		log.Fatal("服务注册失败")
	}
	// 停止服务
	defer reg.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		fmt.Println("服务启动成功", listen.Addr().String())
		if err := rpcServer.Serve(listen); err != nil {
			log.Fatalln(err)
		}
	}()

	// 返回一个chan，当etcd离线后重连机制结束时触发
	go func() {
		<-reg.ListenQuit()
		// TODO 应该在此处理停止应用程序的逻辑
		quit <- syscall.SIGINT
	}()

	<-quit
	fmt.Println("服务关闭成功")
}
