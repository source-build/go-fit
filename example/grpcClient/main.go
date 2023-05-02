package main

import (
	"context"
	"fmt"
	"github.com/source-build/go-fit"
	"github.com/source-build/go-fit/pb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/status"
	"log"
	"time"
)

func main() {
	//连接etcd
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2479"},
		DialTimeout: time.Second * 5,
	})
	if err != nil {
		log.Fatalln(err)
	}

	/* ====== 创建日志追踪 ====== */
	//参数: fileName 需要写入到的日志文件名称，需要预先配置好，不传则不写入到本地日志
	gt := fit.NewLinkTrace()
	//写入方式：LOCAL 本地(NewGinTrace 有参数时才生效) REMOTE 远程 CONSOLE 终端。
	//gt.SetRecordMode("LOCAL")
	//设置服务名称
	gt.SetServiceName("msgpush")
	//设置服务类型，如api服务、rpc服务等
	gt.SetServiceType("api")

	//初始化客户端解析器,全局只能执行一次，例如放到 init 中。
	//发起grpc请求时会自动解析并使用负载均衡策略
	err = fit.NewGrpcClientBuilder(fit.GrpcBuilderConfig{
		EtcdClient:         client,
		ClientCertPath:     "./keys/client.crt",
		ClientKeyPath:      "./keys/client.key",
		RootCrtPath:        "./keys/ca.crt",
		ServerNameOverride: "SourceBuild.cn",
	})
	if err != nil {
		log.Fatalln(err)
	}

	//使用
	// fit.GrpcDial需要搭配etcd使用, serveName是etcd中的key，会以前缀的方式查找key,当查找到多个key时会以轮训的方式选择请求地址。
	// fit.Attempts 与 fit.Rule 二选一, fit.Rule 优先级更高。
	conn, err := fit.GrpcDial(fit.JoinSvsPath("/serves", "rpc", "dpp"),
		fit.Attempts(5),   //重试次数
		fit.WithContext(), //记录一些东西，并写入到日志追踪中
		//fit.Rule(""),      //熔断规则名称，需要提前初始化好，为空则不使用熔断器
	)
	if err != nil {
		log.Fatalln(err)
	}
	defer fit.CloseGrpc(conn)

	check, err := pb.NewPhoneLoginSmsVerCodeClient(conn).Check(context.Background(), &pb.CheckRequest{
		PhoneCode: "2323",
		Code:      1212,
	})
	if err != nil {
		log.Fatalln(status.Convert(err).Message())
	}
	fmt.Println(check.Msg)

	/* 这里以gin为例 */
	//g := gin.New()
	//g.Use(gt.GinTraceHandler())
	//g.GET("/", func(c *gin.Context) {
	//	//传递fit.WithContext()会在拦截器中记录操作信息，耗时等,
	//	conn, err := fit.GrpcDial("/serves/rpc/dpp", fit.Attempts(5), fit.WithContext())
	//	if err != nil {
	//		log.Fatalln(err)
	//	}
	//	defer fit.CloseGrpc(conn)
	//
	//	resp := pb.NewPhoneLoginSmsVerCodeClient(conn)
	//	//想记录rpc调用信息，需要传递context
	//	res, err := resp.SendSteam(c, &pb.CheckRequest{
	//		PhoneCode: "OK",
	//		Code:      200,
	//	})
	//	if err != nil {
	//		log.Fatalln("错误", err)
	//	}
	//	for {
	//		recv, err := res.Recv()
	//		if err == io.EOF {
	//			break
	//		}
	//		if err != nil {
	//			break
	//		}
	//		fmt.Println(recv)
	//	}
	//
	//	c.String(http.StatusOK, "OK")
	//})
	//g.Run(":8005")
}
