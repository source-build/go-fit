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
	err := fit.InitEtcd(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second * 5,
	})
	if err != nil {
		log.Fatalln(err)
	}

	res, err := fit.NewEtcd().Get("foo")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(res)

	//服务发现
	result, err := fit.NewServiceDiscovery(context.Background(), fit.GetClientV3(), "/services/rpc/messagepush/")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(result.SelectByRand())
}
