package main

import (
	"fmt"
	"github.com/source-build/go-fit"
	"log"
)

func main() {
	fit.SetMqURL("amqp://guest:guest@127.0.0.1:5672")
	mq, err := fit.NewRabbitMQ()
	if err != nil {
		log.Fatal(err)
	}
	//释放资源,建议NewRabbitMQ获取实例后 配合defer使用
	defer mq.Close()

	//获取conn
	//mq.Conn()

	//获取channel
	//mq.Channel()

	//(全局生效)设置错误处理方式（默认写入本地日志，不过也需配置本地日志才生效）
	//可传多个 可选值:
	//	- ALL 根据日志配置以所有的方式写入
	//  - LOCAL 仅写入本地日志（需配置）
	//  - REMOTE 仅写入远程日志（需配置）
	//  - CONSOLE 仅将错误输出到控制台
	fit.SetRabbitMqErrLogHandle(fit.ALL)

	//当前实例生效(优先级比全局配置高)
	//mq.SetRabbitMqErrLogHandle(fit.ALL)

	// 声明队列
	// mq.DefQueueDeclare(name,durable) 使用默认声明队列。参数说明: name 队列名称 durable 是否持久化
	// mq.QueueDeclare() 声明队列。跟官方的参数一致，有点多，自己点进去看😊
	// name 为空则随机生成
	// 小贴士: 声明队列支持链式调用,像这样：mq.DefQueueDeclare("logs", false).PublishSimple()
	//mq.DefQueueDeclare("logs", false)

	// 声明交换机
	// mq.DefExchangeDeclare(名称,模式,持久化) 默认交换机。参数模式: 可选值 fit.KIND_*
	// mq.ExchangeDeclare() 跟官方的参数一致，有点多，自己点进去看😊
	// 小贴士: 同样支持链式调用,像这样：mq.DefExchangeDeclare().PublishPub()
	//mq.DefExchangeDeclare("exchange_test", fit.KIND_FANOU,false)

	//******************* （simple|work）简单模式 *******************
	// 注意️： 简单模式(最简单的收发模式)中，不需要用到交换机，所以复制粘贴食用，
	// 消费者多个的情况下消息会以轮询的方式公平分发，每个消费者消费的次数相同。

	//-------------------- 生产者 --------------------
	err = mq.DefQueueDeclare("logs", false).PublishSimple("这是内容")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("发送成功！")

	//-------------------- 消费者 --------------------
	// mq.ConsumeSimple() 使用默认配置创建消费者
	// mq.ConsumeSimple(fit.ConsumeConfig{}) 完整配置创建消费者
	simple, err := mq.ConsumeSimple()
	if err != nil {
		log.Fatal(err)
	}
	for msg := range simple {
		fmt.Println(string(msg.Body))
		//主动应答
		//如果autoAck字段为false(默认)，则需要手动调用msg.Ack(),否则会造成内存溢出
		//如果autoAck字段为true,则服务器将自动确认每条消息，并且不应调用此方法
		err := msg.Ack(true)
		if err != nil {
			log.Fatal("主动应答失败:", err)
		}
	}

	//******************* （publish/subscribe）发布订阅模式 *******************
	//话不多说，这里我就当大家都知道发布订阅模式了
	//生产者发消息broker，由交换机将消息转发到绑定此交换机的每个队列，每个绑定交换机的队列都将接收到消息。

	//-------------------- 生产者(发布) --------------------
	//声明交换机，fit.KIND_FANOUT 表示广播到所有与此绑定的队列
	//err = mq.DefExchangeDeclare("exchange_test1", fit.KIND_FANOUT, false).
	//	PublishPub("这是新的消息") //将消息发送到 exchange_test1 交换机上
	//if err != nil {
	//	log.Fatal(err)
	//}
	//fmt.Println("发布成功")

	//-------------------- 消费者(订阅) --------------------
	//ReceiveSub()方法参数为空则使用默认配置的消费者
	//msgs, err := mq.DefQueueDeclare("", false).
	//	DefExchangeDeclare("exchange_test1", fit.KIND_FANOUT, false).
	//	ReceiveSub()
	//if err != nil {
	//	log.Fatal(err)
	//}
	//for msg := range msgs {
	//	fmt.Println(string(msg.Body))
	//}

	//******************* （routing）路由模式 *******************
	//消息生产者将消息发送给交换机按照路由判断,路由是字符串(info) 当前产生的消息携带路由字符(对象的方法),
	//交换机根据路由的key,只能匹配上路由key对应的消息队列

	//-------------------- 生产者(发布) --------------------
	//声明交换机。fit.KIND_DIRECT 交换机将会对binding key和routing key进行精确匹配，从而确定消息该分发到哪个队列
	//mq = mq.DefExchangeDeclare("exchange_test2", fit.KIND_DIRECT, true)
	////将消息发送到 exchange_test2 交换机上
	//if err := mq.Publish("这是新的消息", "error"); err != nil {
	//	log.Fatal(err)
	//}
	//fmt.Println("发布成功")

	//-------------------- 消费者(接收) --------------------
	//创建交换机
	//ex := mq.DefExchangeDeclare("exchange_test2", fit.KIND_DIRECT, true)
	////随机生成队列名
	//msgs, err = ex.QueueDeclare("", false, false, true, false, nil).
	//	ReceiveRouting("error") //路由key
	//if err != nil {
	//	log.Fatalln(err)
	//}
	//for msg := range msgs {
	//	fmt.Println("来消息了", string(msg.Body))
	//	//主动应答
	//	err := msg.Ack(true)
	//	if err != nil {
	//		log.Fatal("主动应答失败:", err)
	//	}
	//}

	//******************* （topic）主题模式 *******************
	//交换机根据key的规则模糊匹配到对应的队列,由队列的监听消费者接收消息消费
	// - 星号井号代表通配符
	// - 星号代表多个单词,井号代表一个单词
	// - 路由功能添加模糊匹配

	//-------------------- 生产者 --------------------
	//声明交换机。fit.KIND_DIRECT 交换机将会对binding key和routing key进行精确匹配，从而确定消息该分发到哪个队列
	//mq = mq.DefExchangeDeclare("exchange_test3", fit.KIND_TOPIC, true)
	////将消息发送到 exchange_test3 交换机上,注意通配符说明
	////如：hello.* == hello.world | 匹配多个单词: hello.# == hello.world.one
	//if err := mq.PublishTopic("这是新的消息6666", "hello.*"); err != nil {
	//	log.Fatal(err)
	//}
	//fmt.Println("发布成功")

	//-------------------- 消费者 --------------------
	//创建交换机
	//ex := mq.DefExchangeDeclare("exchange_test2", fit.KIND_TOPIC, true)
	////随机生成队列名
	//msgs, err := ex.QueueDeclare("", false, false, true, false, nil).
	//	ReceiveTopic("hello.world")
	//if err != nil {
	//	log.Fatalln(err)
	//}
	//
	//for msg := range msgs {
	//	fmt.Println("来消息了", string(msg.Body))
	//	//主动应答
	//	err := msg.Ack(true)
	//	if err != nil {
	//		log.Fatal("主动应答失败:", err)
	//	}
	//}
}
