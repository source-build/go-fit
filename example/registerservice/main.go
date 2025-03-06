package main

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/source-build/go-fit"
	"github.com/source-build/go-fit/flog"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
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

	flog.Error("", zap.Error(errors.New("错误信息")))

	return

	g := gin.New()

	port, _ := fit.GetFreePort()

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: g,
	}

	reg, err := fit.NewRegisterService(fit.RegisterOptions{
		// 命名空间，默认使用 default
		Namespace: "ht",
		// 服务类型，可选 "api" 与 "rpc"，默认rpc
		ServiceType: "api",
		// 注册中心key，通常为服务名(如user)
		Key: "user",
		// 服务IP，填写 "*" 将自动获取网络出口ip。
		IP: "*",
		// 服务端口
		Port: strconv.Itoa(port),
		// 服务离线时最大重试等待时间，不传则一直阻塞等待，直到etcd恢复
		MaxTimeoutRetryTime: time.Second * 9,
		// etcd 配置
		EtcdConfig: fit.EtcdConfig{
			Endpoints: []string{"127.0.0.1:2379"},
		},
		// zap 日志配置
		Logger: flog.ZapLogger(),
		// 租约时间
		TimeToLive: 10,
		Meta: fit.H{
			"key": "value",
		},
	})
	if err != nil {
		log.Fatal("服务注册失败")
	}

	defer reg.Stop()

	quit := make(chan os.Signal, 1)
	//start service
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server start failed, err:" + err.Error())
		}
	}()

	go func() {
		// 该方法返回一个 chan，当etcd离线后触发重连机制，最终重连失败后返回chan数据
		<-reg.ListenQuit()
		// TODO ... 应该在此处理停止应用程序的逻辑
		fmt.Println("退出")
		quit <- syscall.SIGINT
		fmt.Println("退出1")
	}()

	<-quit
	fmt.Println("退出完成")
}
