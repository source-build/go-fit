package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/source-build/go-fit"
	"gorm.io/gorm"
	"log"
	"net/http"
)

type SystemMenu struct {
	gorm.Model
	Number uint16 `json:"number" gorm:"type:int(1);not null;comment:排序"`
	Name   string `json:"name" gorm:"type:varchar(64);not null;comment:组件名称"`
}

type traceHandler struct {
}

func (t traceHandler) BeforeProcess(trace *fit.Trace) {
	fmt.Println("调用前")
}

func (t traceHandler) AfterProcess(trace *fit.Trace) {
	fmt.Println("调用后")
}

func main() {
	/* 开启本地日志 */
	fit.SetLocalLogConfig(fit.LogEntity{
		LogPath:   "logs",            //修改日志路径，默认为根目录下的logs
		FileName:  "track",           //日志文件名称
		Formatter: fit.JSONFormatter, //格式化方式,不传默认json。可选text(fit.TextFormatter)|json(fit.JSONFormatter)
	})

	//初始化mysql
	//参数2 传的话会记录当次查询的记录，跟着fit.TraceHandler中间件搭配使用
	err := fit.NewMysqlDefConnect(fit.DefaultConfigMysql{
		User: "root",
		Pass: "123456",
		IP:   "127.0.0.1",
		Port: "3316",
		DB:   "system",
	}, true)
	if err != nil {
		log.Fatalln(err)
	}

	//fit.NewMySQL().AutoMigrate(&PhoneSmsVCRecord{})

	//连接redis单节点
	err = fit.NewRedisDefConnect("127.0.0.1:6379", "", "", 0)
	if err != nil {
		log.Fatalln(err)
	}
	defer fit.CloseRedis()

	g := gin.New()
	/* ====== 创建 ====== */
	//参数: 需要写入到的日志文件名称，需要预先配置好, 就是上面的 FileName 字段
	//如果不传则不写入本地日志
	gt := fit.NewLinkTrace("track")
	//写入方式(可写多个): LOCAL 本地 REMOTE 远程 CONSOLE 终端
	gt.SetRecordMode("LOCAL", "REMOTE")
	//设置服务名称
	gt.SetServiceName("user")
	//设置服务类型，如api服务、rpc服务等
	gt.SetServiceType("api")

	//钩子
	//gt.AddHook(new(traceHandler))

	/* gin使用 */
	g.Use(gt.GinTraceHandler())

	//获取上下文
	g.GET("/", func(c *gin.Context) {
		trace, _ := fit.GetGinTraceCtx(c)
		//自定义信息，最终会放到Extend字段下
		trace.Set("name", "张三")
		//可以将行作为日志信息追加到 LogRows 字段下(slice类型)
		trace.AppendLogRow(trace.NewLogInfo(fit.H{"msg": "日志消息1"}))
		trace.AppendLogRow(trace.NewLogInfo(fit.H{"msg": "日志消息2"}))
		trace.AppendLogRow(trace.NewLogInfo(fit.H{"msg": "日志消息3"}))
		c.String(http.StatusOK, "OK")
	})

	/* 记录SQL信息 */
	g.GET("/mysql_gorm", func(c *gin.Context) {
		var system SystemMenu
		//使用WithContext(c)传递上下文，将会记录本次查询的行为
		//需要在初始化mysql时开启才生效
		//fit.TraceCaller() 记录文件名与行数
		fit.MainMysql().Set(fit.TraceCaller()).WithContext(c).Where("id = ?", 9).Take(&system)
		c.String(http.StatusOK, "OK")
	})

	/* 记录Redis信息 */
	g.GET("/redis", func(c *gin.Context) {
		//使用fit.WithGinTraceCtx(c)传递当前context,会收集本次操作的信息
		fit.MainRedis(fit.WithGinTraceCtx(c)).Get("USER:USER_LOGIN:ID:1")
		c.String(http.StatusOK, "OK")
	})

	g.Run(":8003")
}
