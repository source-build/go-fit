package main

import (
	"fmt"
	"github.com/source-build/go-fit"
)

func main() {
	/* 基本使用 */
	//开启本地日志
	fit.SetLocalLogConfig(fit.LogEntity{
		LogPath:      "./logs",    //修改日志路径，默认为根目录下的logs
		FileName:     "diagnosis", //日志文件名称
		IsDefaultLog: true,        //默认日志，当直接调用Error、Info时会选择默认的
	}, fit.LogEntity{
		LogPath:  "./logs", //修改日志路径，默认为根目录下的logs
		FileName: "track",  //日志文件名称
	})

	//设置堆栈错误信息长度(默认300)，错误信息key应为err
	fit.SetLogStackLength(100)
	//开启控制台输出
	fit.SetOutputToConsole(true)
	//开启远程支持，这里使用rabbitMQ并使用路由模式
	fit.SetMqURL("amqp://guest:guest@110127.0.0.1:5672")
	fit.SetRemoteRabbitMQLog(&fit.RemoteRabbitMQLog{ //
		Exchange: "abnormalHandle", //交换机名称
		Key:      "error",          //key，与此名称绑定的队列才能消费消息
		Durable:  true,             //交换机持久化
	})
	//自定义错误处理
	c := fit.CustomizeLog()
	//关闭管道
	defer fit.CloseCustomizeLog()
	for msg := range c {
		fmt.Println("错误信息：", msg)
	}
	//使用
	fit.Error("这是错误信息")

	/* 使用指定实例日志 */
	//参数1: 日志文件名称，也就是FileName字段
	//参数2: 配置
	//	fit.UseConsole() 输出到控制台
	//	fit.UseLocal()   输出到本地文件
	//	fit.UseRemote()  输出到远程mq
	//  fit.UseReportCaller(true) 记录文件名，行数
	//  fit.UseSetSkip(2) 上溯的栈帧数,输出发生错误的位置，包括文件名和行数，参数为 栈帧数。fit.UseReportCaller(true) 时有效
	fit.UseOtherLog("track", fit.UseLocal()).Error("这是信息消息")

	///**
	//当只有一个参数时,会以字符串格式的形式写入
	//当参数超过2个时，会以json的形式写入，如: fit.Error("name","张三","age":18) -> {"name":"张三","age":18}
	//*/
	////注意：当 err 为key时，value 应为 error 类型,否则将输出错误信息到本地日志
	//fit.Error("err", errors.New("获取用户信息失败")) //错误级别
	//fit.Warning("错误信息")                      //警告级别
	//fit.Fatal("错误信息")                        //崩溃级别,会停止程序
	//
	///* 仅写入本地日志,不受全局配置的影响，但需要 开启本地日志 */
	//fit.LocalLog().Info("error info")
	//
	///* 仅使用远程配置,不受全局配置的影响，但需要 开启远程支持 */
	//// 第一个参数是类型，当远程写入失败时会将错误信息写入本地
	//// 第二个参数跟 Error Warning Fatal 用法一致
	//fit.RemoteLog(fit.ErrorLevel, "err", "获取用户信息失败")
	//
	///* 其他用法 */
	//s := fit.Fields{"key": "value"}.ToSlice()
	//fit.Error(s...)
}
