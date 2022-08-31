/**
service register
*/
package main

import (
	"context"
	"github.com/source-build/go-fit"
	"go.etcd.io/etcd/client/v3"
	"log"
	"time"
)

func main() {
	//连接etcd
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second * 5,
	})
	//服务注册
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	s, err := fit.NewServiceRegister(&fit.ServiceRegister{
		Ctx:    ctx,
		Client: client,
		Key:    "foo/user/192.168.1.6:8080",
		Value:  "192.168.1.6:8080",
		Lease:  10,
	})
	if err != nil {
		log.Fatalln(err)
	}
	defer s.Close()
}
