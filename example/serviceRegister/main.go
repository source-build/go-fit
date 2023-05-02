/**
服务注册
*/
package main

import (
	"context"
	"github.com/source-build/go-fit"
	"go.etcd.io/etcd/client/v3"
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

	addr, _ := fit.GetRandomAvPortAndHost()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := make(chan os.Signal)
	s, err := fit.NewServiceRegister(&fit.ServiceRegister{
		Ctx:        ctx,
		Client:     client,
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

	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	<-c
	s.Close()
}
