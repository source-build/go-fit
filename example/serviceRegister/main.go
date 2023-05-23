package main

import (
	"context"
	"fmt"
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

//服务注册

func main() {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second * 60,
		DialOptions: []grpc.DialOption{
			grpc.WithBlock(),
		},
	})

	defer client.Close()

	completeChan := make(chan struct{}, 1)
	defer close(completeChan)

	//创建一个计数器
	stat := fit.NewStatUnfinished(&fit.StatUnfinished{Signal: completeChan})

	/* gin 使用 */
	g := gin.New()
	g.Use(stat.GinStatUnfinished())

	/* grpc 使用 */
	var opts []grpc.ServerOption

	//日志收集
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
	go func() {
		var a string
		for {
			fmt.Scanf("输入:%s", &a)
			fmt.Println(333)
			c <- os.Interrupt
		}
	}()
	s, err := fit.NewServiceRegister(&fit.ServiceRegister{
		Ctx:    ctx,
		Client: client,

		//重试次数。到达指定次数仍无法连接的，向 c 写入中断信号。
		RetryCount: 5,
		//重试回调, count:当前重试次数。
		RetryFunc: func(count int) {},
		//重试成功回调。
		RetryOkFunc: func() {},
		//重试间隔时间,默认 5s。
		//RetryWaitDuration: time.Second * 10,
		//重试间隔时间是上一次两倍
		//RetryWaitMultiple: true,

		// 避免key冲突(仅 fit.EnvDevelopment(开发环境) 有效)。
		// 当多人协同开发时，由于可能共用的是同一个etcd而开发环境又处于不同的局域网之中，在服务注册时可能会导致key被覆盖。
		// 如果启用,在服务注册时会在key中加一层字符串,这个字符串可以理解为你的机器码,这样在服务发现时就只会寻找和本机有关的key。
		// *注意： 在生产环境中不应该使用它。
		UseIsolate: true,
		Env:        fit.EnvDevelopment,

		//Key 命名建议
		// --> /项目名/svs/服务类型/服务名称
		// 默认会在服务后面生成6位数的随机字符,因为单个服务可能会启动多个进程监听不同的端口已达到负载均衡的效果。
		// 如果你想将完整的字符串作为服务在注册中心的key,那么使用`NoSuffix:true`关闭它,它将不会再生成随机后缀。
		Key:   "/ht/svs/api/test_user",
		Value: fit.NewRegisterCenterValue(addr),
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
		Lease:      15,
		SignalChan: c, //传递一个chan，当退出时会向其写入信号，默认为 os.Interrupt
		SignalTag:  os.Kill,
		//当etcd离线或key失效时触发
		OnBack: func() {},
	})
	if err != nil {
		log.Fatalln(err)
	}

	<-c
	s.Close() //这里是关闭资源而不是关闭etcd客户端，注意调用顺序。
}
