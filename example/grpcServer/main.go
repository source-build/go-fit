package main

import (
	"context"
	"fmt"
	"github.com/source-build/go-fit"
	"github.com/source-build/go-fit/pb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type phoneSms struct {
	pb.UnimplementedPhoneLoginSmsVerCodeServer
}

func (p phoneSms) Send(ctx context.Context, request *pb.SendRequest) (*pb.Response, error) {
	trace, ok := fit.GetTraceCtx(ctx)
	if ok {
		fmt.Println(trace.TraceId)
	}
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
		Result: "OK",
	}, nil
}

func main() {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2479"},
		DialTimeout: time.Second * 5,
	})

	listen, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalln(err)
	}

	//create tls
	cred, err := fit.NewServiceTLS(&fit.CertPool{
		CertFile: "../../keys/server.crt",
		KeyFile:  "../../keys/server.key",
		CaCert:   "../../keys/ca.crt",
	})
	if err != nil {
		fit.Fatal("create tls failed err:" + err.Error())
	}

	///* 开启本地日志 */
	//fit.SetLocalLogConfig(fit.LogEntity{
	//	LogPath:      "./logs",          //修改日志路径，默认为根目录下的logs
	//	FileName:     "track",           //日志文件名称
	//	Formatter:    fit.JSONFormatter, //格式化方式,不传默认json。可选text(fit.TextFormatter)|json(fit.JSONFormatter)
	//})
	//
	///* ====== 创建 ====== */
	////参数: 需要写入到的日志文件名称，需要预先配置好, 说白了就是上面的 FileName 字段
	////如果不传则不写入本地日志
	//gt := fit.NewLinkTrace("track")
	////写入方式：LOCAL 本地(NewGinTrace 有参数时才生效) REMOTE 远程 CONSOLE 终端。
	//gt.SetRecordMode("LOCAL")
	////设置服务名称
	//gt.SetServiceName("user")
	////设置服务类型，如api服务、rpc服务等
	//gt.SetServiceType("rpc")
	//
	var opts []grpc.ServerOption
	//
	opts = append(opts, grpc.Creds(cred))

	addr, _ := fit.GetRandomAvPortAndHost()

	//日志收集
	//由于只能设置一个拦截器，如果你也想使用拦截器，则需要添加一个hook
	//gt.GrpcHook(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	//	//如果不调用handler，将不会继续往下处理
	//	res, err := handler(ctx, req)
	//	return res, err
	//})
	//注意：这是一元拦截器
	//****
	//opts = append(opts, grpc.UnaryInterceptor(gt.GrpcServerInterceptor()))

	rpcServer := grpc.NewServer(opts...)

	pb.RegisterPhoneLoginSmsVerCodeServer(rpcServer, new(phoneSms))

	//服务注册
	quit := make(chan os.Signal, 1)
	s, err := fit.NewServiceRegister(&fit.ServiceRegister{
		Ctx:        context.Background(),
		Client:     client,
		Key:        "/serves/rpc/dpp/" + fit.NewRandom().Char(5),
		Value:      fit.NewRegisterCenterValue(addr),
		Lease:      20,
		SignalChan: quit,
	})
	if err != nil {
		log.Fatalln(err)
	}

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		fmt.Println("服务启动成功!", addr)
		if err := rpcServer.Serve(listen); err != nil {
			log.Fatalln(err)
		}
	}()
	<-quit
	s.Close()
	fmt.Println("service close!")
}
