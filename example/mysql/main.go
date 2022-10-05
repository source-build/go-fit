package main

import (
	"github.com/source-build/go-fit"
	"gorm.io/gorm"
	"log"
	"time"
)

func main() {
	//使用默认的方式连接
	//参数2 传的话会记录当次查询的记录，跟着trace中间件搭配使用
	err := fit.NewMysqlDefConnect(fit.DefaultConfigMysql{
		User: "root",
		Pass: "123",
		IP:   "192.168.1.6",
		Port: "3369",
		DB:   "foo",
	}, false)
	if err != nil {
		log.Fatalln(err)
	}

	//自定义配置的方式连接
	addr := "root:123@tcp(127.0.0.1:3369)/foo?charset=utf8mb4&parseTime=True&loc=Local"
	pool, err := fit.NewMysqlConnect(addr, &gorm.Config{}, true, false)
	if err != nil {
		log.Fatalln(err)
	}
	defer pool.Close()

	//设置空闲连接池中的最大连接数
	pool.SetMaxIdleConns(10)
	//设置打开数据库连接的最大数量
	pool.SetMaxOpenConns(200)
	//设置连接可复用的最大时间。
	pool.SetConnMaxLifetime(time.Hour)

	//使用
	//fit.MainMysql().Create()
}
