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

	//服务发现
	result, err := fit.NewServiceDiscovery(context.Background(), fit.MainEtcdClientv3(), "/foo/user/")
	if err != nil {
		log.Fatalln(err)
	}
	// result.SelectByRand() 随机取一项
	value, err := result.ParseValue(result.SelectByRand()) //提取
	fmt.Println(err, value.Addr)
}
