package main

import (
	"fmt"
	"github.com/nsqio/go-nsq"
	"github.com/source-build/go-fit"
	"os"
	"os/signal"
	"syscall"
)

type MyHandler struct {
}

func (m *MyHandler) HandleMessage(message *nsq.Message) error {
	fmt.Println(string(message.Body))
	return nil
}

func main() {
	var handler MyHandler
	config := fit.ConsumerEntity{
		Topic:   "abnormal",
		Channel: "abnormalError",
		Address: "127.0.0.1:4161",
		Handler: &handler,
	}
	err := fit.InitConsumer(config)
	if err != nil {
		fmt.Printf("init consumer failed, err:%v\n", err)
		return
	}
	c := make(chan os.Signal)        // 定义一个信号的通道
	signal.Notify(c, syscall.SIGINT) // 转发键盘中断信号到c
	<-c
}
