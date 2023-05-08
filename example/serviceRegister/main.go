/**
服务注册
*/
package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/source-build/go-fit"
	"go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2479"},
		DialTimeout: time.Second * 5,
	})

	completeChan := make(chan struct{}, 1)
	defer close(completeChan)

	//创建一个计数器
	stat := fit.NewStatUnfinished(&fit.StatUnfinished{Signal: completeChan})

	/* gin 使用 */
	g := gin.New()
	g.Use(stat.GinStatUnfinished())

	/* grpc 使用 */
	var opts []grpc.ServerOption

	//使用日志收集
	//由于只能设置一个拦截器，如果想使用拦截器，需要添加一个hook
	gt := fit.NewLinkTrace()
	gt.GrpcHook(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := stat.GrpcHandleStatUnfinished(); err != nil {
			return nil, err
		}

		stat.Add()
		res, err := handler(ctx, req)
		stat.Sub()

		return res, err
	})
	//opts = append(opts, grpc.UnaryInterceptor(gt.GrpcServerInterceptor()))

	//不使用日志收集的话直接使用拦截器
	opts = append(opts, grpc.UnaryInterceptor(stat.GrpcStatUnfinished()))

	grpc.NewServer(opts...)

	//stat.Value() 查看当前还有多少未完成的请求 0表示当前无请求
	//stat.FiringWaitDone() //拦截请求，返回http状态码 400
	//stat.Restore()        //恢复处理请求

	addr, _ := fit.GetRandomAvPortAndHost()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	s, err := fit.NewServiceRegister(&fit.ServiceRegister{
		Ctx:    ctx,
		Client: client,
		OnStatusChange: func(value fit.RegisterCenterValue, this *fit.ServiceRegister) {
			// 关闭指令。等待所有请求完成后调用 fit.Shutdown() 关闭服务
			// 最终状态，不建议再修改状态
			if value.Status == fit.ServiceStatusWaitDone {
				// TODO ...等待正在进行的请求处理完成
				stat.FiringWaitDone() //拦截请求
				<-completeChan
				this.Shutdown()
			}

			// 服务不可用指令。可以将状态重新恢复，但不要立马恢复
			if value.Status == fit.ServiceStatusNotAvailable {
				//设置服务为不可用
				stat.SetAvailable(false)

				// 建议根据不可用原因分析原因，等待一段时间，若立刻恢复，那么触发函数将毫无意义。
				time.Sleep(time.Second * 5)

				//继续提供服务
				stat.SetAvailable(true)

				// 恢复服务，状态变成 fit.ServiceStatusRun
				if err := this.Restore(value); err != nil {
					log.Println(err)
					return
				}
			}
		},
		Key:        "/foo/user/192.168.1.6:8080",
		Value:      fit.NewRegisterCenterValue(addr),
		Lease:      15,
		SignalChan: c, //可传递一个chan，当etcd离线或key失效时会向其chan写入信号，默认为 os.Kill
		SignalTag:  os.Kill,
		OnBack:     func() {}, //当etcd离线或key失效时触发
	})
	if err != nil {
		log.Fatalln(err)
	}

	<-c
	s.Close()
}
