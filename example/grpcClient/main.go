package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/source-build/go-fit"
	"github.com/source-build/go-fit/pb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"io"
	"log"
	"net/http"
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

	/* ====== 创建 ====== */
	//参数: 需要写入到的日志文件名称，需要预先配置好, 说白了就是上面的 FileName 字段
	//如果不传则不写入本地日志
	gt := fit.NewLinkTrace()
	//写入方式：LOCAL 本地 REMOTE 远程 CONSOLE 终端。NewGinTrace 有参数时才生效
	//gt.SetRecordMode("LOCAL")
	//设置服务名称
	gt.SetServiceName("msgpush")
	//设置服务类型，如api服务、rpc服务等
	gt.SetServiceType("api")

	//初始化客户端解析器
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

	g := gin.New()
	g.Use(gt.GinTraceHandler())

	g.GET("/", func(c *gin.Context) {
		//传递fit.WithContext()会在拦截器中记录操作信息，耗时等,
		//注意 在调用rpc方法时还需要传入context
		conn, err := fit.GrpcDial("/serves/rpc/dpp",
			fit.Attempts(5),
			fit.WithContext(),
		)
		if err != nil {
			log.Fatalln("连接失败", err)
		}
		defer conn.Close()

		resp := pb.NewPhoneLoginSmsVerCodeClient(conn)
		//想记录rpc调用信息，需要传递context
		res, err := resp.SendSteam(c, &pb.CheckRequest{
			PhoneCode: "OK",
			Code:      200,
		})
		if err != nil {
			log.Fatalln("错误", err)
		}
		for {
			recv, err := res.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			fmt.Println(recv)
		}

		c.String(http.StatusOK, "OK")
	})
	g.Run(":8005")
}
