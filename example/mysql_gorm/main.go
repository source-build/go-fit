package main

import (
	"github.com/source-build/go-fit"
	"gorm.io/gorm"
	"log"
	"time"
)

func main() {
	fit.SetLogLevel(fit.InfoLevel)
	fit.SetLocalLogConfig(fit.LogEntity{
		LogPath:  "logs",
		FileName: "diagnosis",
	})
	fit.SetOutputToConsole(true)

	instance, ok := fit.GetLogInstance("diagnosis")
	if !ok {
		log.Fatalln("not find")
	}

	//使用默认的方式连接
	//参数2 记录操作,需要与trace中间件搭配使用
	err := fit.NewMysqlDefConnect(fit.DefaultConfigMysql{
		User:      "root",
		Pass:      "12345678",
		IP:        "127.0.0.1",
		Port:      "3306",
		DB:        "",
		FitLogger: instance, //输出到 diagnosis.log 中,Debug 级别中有效
		//Logger: logger.New(), // 自定义日志
		//LogMode: logger.Error, // 自定义日志级别 默认 Error,仅 Logger 存在时有效;
	}, false)
	if err != nil {
		log.Fatalln(err)
	}

	//自定义配置的方式连接
	addr := "root:123@tcp(127.0.0.1:3369)/foo?charset=utf8mb4&parseTime=True&loc=Local"
	pool, err := fit.NewMysqlConnect(addr, &gorm.Config{}, false)
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
	//fit.MainMysql()

	//推荐错误处理
	//先使用fit.HandleGormQueryErrorFromTx 或 fit.HandleGormQueryError 检查一下是不是mysql错误,
	//因为 gorm 查询不到记录时也会报 gorm.ErrRecordNotFound 错误,导致在开发中需要多判断一次完全没必要,
	//先使用以上两个方法判断,如果返回nil,那么直接使用RowsAffected判断。
	//
	//对于更新、创建、删除操作,直接判断错误。
	var count int64
	tx, err := fit.HandleGormQueryErrorFromTx(fit.MainMysql().Table("users").Where("gender = 1").Count(&count))
	if err != nil {
		return
	}
	if tx.RowsAffected == 0 {
		// ...No data
	}
}
