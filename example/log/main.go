package main

import (
	"errors"
	"fmt"
	"github.com/source-build/go-fit"
	"log"
)

type remoteLogHook struct {
}

func (r remoteLogHook) Before(body map[string]interface{}) map[string]interface{} {
	// TODO 发送之前做点什么
	body["key"] = "foo"
	return body
}

func (r remoteLogHook) Error(err error) {
	// TODO 发生错误时做点什么
	// err
}

func main() {
	fit.SetLogLevel(fit.DebugLevel)
	fit.SetLocalLogConfig(fit.LogEntity{
		LogPath:  "logs",      //日志文件存放的路径，默认 logs;
		FileName: "diagnosis", //日志文件名称,默认名称:"general.log"
	})
	fit.SetOutputToConsole(true)

	fit.Error(errors.New("这是此哦污嘻嘻"))
	fit.Warning("哈哈哈")
	fit.Info("哈哈哈")
	fit.Debug("哈哈哈")
	fit.ErrorJSON(fit.H{"title": "666"})
}

func main11() {
	//设置日志级别,需要在SetLocalLogConfig之前设置
	//注意：级别顺序为, debug < info < warning < error
	//如果级别为debug,那么会输出所有级别(开发环境)
	//例如级别为warning,那么只会输出更高级别的日志(warning、error),以此类推
	//开发环境可设置为debug,生产环境info(默认级别)
	fit.SetLogLevel(fit.InfoLevel)

	/* 开启本地日志 */
	//🙅 注意,多实例日志会增加磁盘IO开销,谨慎使用
	fit.SetLocalLogConfig(fit.LogEntity{
		LogPath:  "logs",      //日志文件存放的路径，默认 logs;
		FileName: "diagnosis", //日志文件名称,默认名称:"general.log"

		//关闭记录文件名-位置,默认开启,输出到 caller 字段;
		//ReportCaller: true,

		//默认日志,当直接调用fit.Error、fit.Info...时会使用的日志实例;
		//当 fit.LogEntity 只有一项时,默认日志就是第一项,无需传入 IsDefaultLog;
		//IsDefaultLog: true,
	},
	//多实例
	//fit.LogEntity{
	//	LogPath:  "logs",
	//	FileName: "track",
	//}, fit.LogEntity{
	//	LogPath:  "logs",
	//	FileName: "mysql_gorm",
	//}
	)

	/* 设置堆栈错误信息长度(默认300) */
	fit.SetLogStackLength(100)
	/* 开启控制台输出,仅 Debug 级别有效 */
	fit.SetOutputToConsole(true)
	/* 禁用控制台彩色字体输出 */
	fit.SetConsoleLogNoColor()

	/* ============== 开启远程日志，使用rabbitMQ的routing模式，消息格式:json(可通过hook函数来修改) ============== */
	/******** 参数 Simple=true 的情况下 : ******/
	// 最高优先级。
	// Kind 参数失效，不再使用 routing 模式，而是使用 Simple 模式，
	// 并且将 Key 作为队列名称。
	// 接收消息代码参考: simple, err := mq.DefQueueDeclare("logs", false, true).ConsumeSimple()

	/******** 参数 Simple=false 的情况下 : ******/
	// ❌ 如果消息发送到交换器时没有与此交换器绑定的队列，那么这个消息将被丢弃。

	/******** 参数使用 fit.KIND_DIRECT 的情况下: ******/
	// Key 参数失效。
	// 当错误被触发时，会按照错误级别发送到指定的队列中，如：Error 级别的日志会使用 error 作为RoutingKey，
	// 也就意味着,消费者需要使用 ReceiveRouting("error") 来接收消息。同理其他级别也是一样的，分别有 debug、info、warning、error、fatal。
	// 接收消息代码参考(空队列名表示生成随机名称的队列):
	// msgs := mq.DefExchangeDeclare("app_logs", fit.KIND_DIRECT, true, false).QueueDeclare("", false, false, false, false, nil)
	// msgs.ReceiveRouting("error") //接收错误级别的日志消息
	// msgs.ReceiveRouting("info") //接收消息级别的日志消息...

	/******** 参数是非 fit.KIND_DIRECT 的情况下: ******/
	// Key 参数生效。Kind 参数失效。会将 Key 作为 RoutingKey，且强制将 Kind 参数设置为 fit.KIND_FANOUT。

	// 🔔 注意: 写入远程RabbitMQ时并不会频繁地创建连接，内部维护一个状态，当写入远程时会刷新最新时间，当最后一条连接10秒后还未被使用，那么将断开连接，关闭状态机。
	// 换句话说，10秒内如果至少被触发了一次写入远程日志(fit.Error();这样的算一次)，那么连接就不会被销毁，当然，你也可以通过 MaxConnAt 字段来设置最大保持时间。

	fit.SetMqURL("amqp://guest:guest@127.0.0.1:5672") //全局设置RabbitMQ地址
	fit.SetRemoteRabbitMQLog(&fit.RemoteRabbitMQLog{
		//RabbitMQUrl: "",               //单独设置RabbitMQ地址，优先级大于 全局设置（即 fit.SetMqURL）
		Exchange: "exchange_test3", //交换机名称，Simple = true时失效。
		Simple:   true,             //是否使用简单模式,Kind 将失效, Key 将作为队列的 Name(默认 false)。
		Key:      "app_logs",       //routingKey。如果不使用Simple模式并且使用KIND_DIRECT,那么与此名称绑定的队列才能消费消息。

		//fit.KIND_DIRECT 交换器将会对bindingKey和routingKey进行精确匹配,从而确定消息该分发到哪个队列(推荐)。
		//fit.KIND_FANOUT 交换器将广播到所有与此绑定的队列。
		Kind:    fit.KIND_DIRECT,
		Durable: false, //交换器持久化

		//自动删除。该功能必须要在交换器曾经绑定过队列或者交换器的情况下，处于不再使用的时候才会自动删除。
		AutoDel: true,

		//最大保持连接时长，0表示不设置(如果一直被使用，那么该连接将不会被销毁)，单位/秒。
		//如果需要设置，建议增加时长(例如:>1天)，这个机制的目的就是防止频繁的创建连接，如果时长较短，那将毫无意义。
		//MaxConnAt: 60*60*24,
		MaxConnAt: 0,
	})

	/* 输出到指定的日志文件 */
	//name: 日志文件名称，也就是配置时的FileName字段
	//opts:
	//	fit.UseConsole() 输出到控制台
	//	fit.UseLocal()   输出到本地文件
	//	fit.UseRemote()  输出到远程mq
	//  fit.UseNotReportCaller() 不记录文件名\行数,默认记录。
	//  fit.UseSetSkip(2) 上溯的栈帧数,输出发生错误的位置，包括文件名和行数，参数为 栈帧数。fit.UseReportCaller(true) 时有效
	fit.OtherLog("track", fit.UseLocal()).Error("这是信息消息")

	/*只写入本地而且不受全局配置的影响，可以使用以下方式，前提需要开启本地日志*/
	//若不传递参数,则默认选择第一个日志实例
	fit.LocalLog("track").Info("error info")

	/*只写入远程而且不受全局配置的影响，可以使用以下方式，不过还是需要开启远程支持*/
	// 第一个参数是日志类型，当远程写入失败时会将错误信息写入本地
	// 剩余参数跟 Error Warning Fatal 用法一致
	fit.RemoteLog(fit.ErrorLevel, "msg", "获取用户信息失败", "err", "err info")

	/* 在远程日志发送之前做点什么? */
	fit.AddRemoteLogHook(new(remoteLogHook))

	/* 自定义错误处理 */
	go func() {
		c := fit.CustomizeLog()
		defer fit.CloseCustomizeLog()
		for msg := range c {
			fmt.Println("错误信息：", msg)
		}
	}()

	//获取logrus实例
	fit.GetLogInstances()
	instance, ok := fit.GetLogInstance("mysql_gorm")
	if !ok {
		log.Fatalln("not find")
	}
	instance.Error()

	/*快捷使用*/
	//参数可以只传一个,或者必须是偶数
	//可以直接传入一个err,会被记录到"err"字段中
	fit.Error(errors.New("error info"))
	fit.Debug("content")   //Debug
	fit.Info("content")    //消息
	fit.Warning("content") //警告
	fit.Error("content")   //错误
	fit.Fatal("content")   //致命的

	//会将结果输出到json字段中
	fit.ErrorJSON(fit.H{"title": "666"})
	fit.WarningJSON(fit.H{"title": "666"})
	fit.InfoJSON(fit.H{"title": "666"})
	fit.FatalJSON(fit.H{"title": "666"})

	/* 其他用法 */
	fit.Error(fit.Fields{"key": "value"}.ToSlice()...)
}
