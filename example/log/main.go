package main

import (
	"fmt"
	"github.com/source-build/go-fit"
	"log"
)

type remoteLogHook struct {
}

func (r remoteLogHook) Before(body string) string {
	return body
}

func (r remoteLogHook) Error(err error) {
	fmt.Println("当发生错误时触发", err)
}

func main() {
	/* 配置 */
	//开启本地日志
	fit.SetLocalLogConfig(fit.LogEntity{
		LogPath:      "logs",      //日志文件存放的路径，默认为根目录下的logs
		FileName:     "diagnosis", //日志文件名称
		IsDefaultLog: true,        //默认日志，当直接调用Error、Info时会选择默认的
	}, fit.LogEntity{
		LogPath:  "logs",
		FileName: "track",
	}, fit.LogEntity{
		LogPath:  "logs",
		FileName: "mysql",
	})
	//设置堆栈错误信息长度(默认300)，错误信息key应为err
	fit.SetLogStackLength(100)
	//开启控制台输出
	fit.SetOutputToConsole(true)

	//开启远程日志，这里使用rabbitMQ并使用路由模式，消息会原样发送
	fit.SetMqURL("amqp://guest:guest@127.0.0.1:5672")
	fit.SetRemoteRabbitMQLog(&fit.RemoteRabbitMQLog{ //
		Exchange: "abnormalHandle", //交换机名称
		Key:      "error",          //key，与此名称绑定的队列才能消费消息
		Durable:  true,             //交换机持久化
	})

	//如果你想使用指定
	//参数1: 日志文件名称，也就是 开启本地日志 的FileName字段
	//参数2: 配置
	//	fit.UseConsole() 输出到控制台
	//	fit.UseLocal()   输出到本地文件
	//	fit.UseRemote()  输出到远程mq
	//  fit.UseReportCaller(true) 记录文件名，行数
	//  fit.UseSetSkip(2) 上溯的栈帧数,输出发生错误的位置，包括文件名和行数，参数为 栈帧数。fit.UseReportCaller(true) 时有效
	fit.OtherLog("track", fit.UseLocal()).Error("这是信息消息")

	//如果你只想写入本地而且不受全局配置的影响，可以使用以下方式，不过还是需要开启本地日志
	//如果有参数，则会使用指定的日志实例写入，需要在 开启本地日志
	fit.LocalLog("track").Info("error info")

	//如果你只想使用远程而且不受全局配置的影响，可以使用以下方式，不过还是需要开启远程支持
	// 第一个参数是日志类型，当远程写入失败时会将错误信息写入本地
	// 第二个参数跟 Error Warning Fatal 用法一致
	fit.RemoteLog(fit.ErrorLevel, "msg", "获取用户信息失败")

	//在远程日志发送之前做点什么?
	fit.AddRemoteLogHook(new(remoteLogHook))

	//自定义错误处理
	go func() {
		c := fit.CustomizeLog()
		defer fit.CloseCustomizeLog()
		for msg := range c {
			fmt.Println("错误信息：", msg)
		}
	}()

	//获取logrus实例
	fit.GetLogInstances()
	instance, ok := fit.GetLogInstance("mysql")
	if !ok {
		log.Fatalln("not find")
	}
	instance.Error()

	//快捷使用
	fit.Error()   //错误
	fit.Warning() //警告
	fit.Info()    //消息
	fit.Fatal()   //致命的

	fit.ErrorJSON(fit.H{"title": "666"})
	fit.WarningJSON(fit.H{"title": "666"})
	fit.InfoJSON(fit.H{"title": "666"})
	fit.FatalJSON(fit.H{"title": "666"})

	/* 其他用法 */
	s := fit.Fields{"key": "value"}.ToSlice()
	fit.Error(s...)
}
