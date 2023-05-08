package main

import (
	"context"
	"fmt"
	"github.com/source-build/go-fit"
	"go.etcd.io/etcd/client/v3"
	"log"
	"time"
)

func main() {
	//连接
	err := fit.InitEtcd(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2479"},
		DialTimeout: time.Second * 5,
	})
	if err != nil {
		log.Fatalln(err)
	}

	defer fit.CloseEtcd()

	//服务发现
	result, err := fit.NewServiceDiscovery(context.Background(), fit.MainEtcdClientv3(), "/foo/user/")
	if err != nil {
		log.Fatalln(err)
	}
	// result.SelectByRand() 随机取一个服务
	value, err := result.SelectByRand()
	if err != nil {
		// err 服务不可用原因
		log.Fatalln(err)
	}

	fmt.Println(value.Addr)
}
