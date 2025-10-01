package main

import (
	"log"

	"github.com/source-build/go-fit"
)

func main() {
	// 全局设置
	fit.GlobalSetRabbitMQUrl("amqp://guest:guest@127.0.0.1:5672")
	//mq, err := fit.NewRabbitMQ()

	//单独设置rabbitMQ地址
	mq, err := fit.NewRabbitMQ("amqp://guest:guest@127.0.0.1:5672")
	if err != nil {
		log.Fatal(err)
	}
	//释放资源
	defer mq.Close()
}
