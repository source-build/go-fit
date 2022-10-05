package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/source-build/go-fit"
	"gorm.io/gorm"
	"net/http"
)

type User struct {
	gorm.Model
	NickName string `json:"nick_name" gorm:"type:varchar(15);comment:昵称"`
}

type traceHandler struct {
}

func (t traceHandler) BeforeProcess(trace *fit.Trace) {
	fmt.Println("调用前")
}

func (t traceHandler) AfterProcess(trace *fit.Trace) {
	fmt.Println("调用后")
}

type PhoneSmsVCRecord struct {
	gorm.Model
	Phone      string `json:"phone" gorm:"type:varchar(11);comment:手机号"`
	TemplateId string `json:"template_id" gorm:"type:varchar(11);comment:短信模版id"`
	Code       string `json:"code" gorm:"type:varchar(6);comment:验证码"`
	Pin        string `json:"pin" gorm:"type:varchar(30);comment:校验值"`
}

func main() {
	/* 开启本地日志 */
	fit.SetLocalLogConfig(fit.LogEntity{
		LogPath:      "./logs",          //修改日志路径，默认为根目录下的logs
		FileName:     "track",           //日志文件名称
		Formatter:    fit.JSONFormatter, //格式化方式,不传默认json。可选text(fit.TextFormatter)|json(fit.JSONFormatter)
		IsDefaultLog: true,
		ReportCaller: true, //输出文件名 行数, IsDefaultLog = true 时生效
	})

	//初始化mysql
	//参数2 传的话会记录当次查询的记录，跟着fit.TraceHandler中间件搭配使用
	//err := fit.ConnectDefaultConfigMysql(fit.DefaultConfigMysql{
	//	User: "grxc",
	//	Pass: "445566",
	//	IP:   "110.42.184.124",
	//	Port: "3316",
	//	DB:   "messagepush",
	//}, true)
	//if err != nil {
	//	log.Fatalln(err)
	//}

	//fit.NewMySQL().AutoMigrate(&PhoneSmsVCRecord{})

	//连接redis单节点
	//err = fit.NewDefaultRedisClient("127.0.0.1:6380", "", "", 0)
	//if err != nil {
	//	log.Fatalln(err)
	//}
	//defer fit.CloseRedis()

	g := gin.New()
	/* ====== 创建 ====== */
	//参数: 需要写入到的日志文件名称，需要预先配置好, 说白了就是上面的 FileName 字段
	//如果不传则不写入本地日志
	gt := fit.NewLinkTrace("track")
	//写入方式：LOCAL 本地 REMOTE 远程 CONSOLE 终端。NewGinTrace 有参数时才生效
	gt.SetRecordMode("LOCAL")
	//设置服务名称
	gt.SetServiceName("user")
	//设置服务类型，如api服务、rpc服务等
	gt.SetServiceType("api")

	//钩子
	gt.AddHook(new(traceHandler))

	//使用
	g.Use(gt.GinTraceHandler())

	//获取上下文
	g.GET("/", func(c *gin.Context) {
		trace, _ := fit.GetGinTraceCtx(c)
		//自定义信息，最终会放到Extend字段下
		trace.Set("name", "zhangsan")
		c.String(http.StatusOK, "OK")
	})

	/* 记录SQL信息 */
	g.GET("/mysql", func(c *gin.Context) {
		var user User
		//使用WithContext(c)传递上下文，将会记录本次查询的行为
		//不过需要在初始化mysql时开启才生效
		//fit.TraceCaller() 记录文件名与行数
		fit.MainMysql().Set(fit.TraceCaller()).WithContext(c).Where("id = ?", 9).Take(&user)
		c.String(http.StatusOK, "OK")
	})

	/* 记录Redis信息 */
	//g.GET("/redis", func(c *gin.Context) {
	//	//使用fit.WithGinTraceCtx(c)传递当前context,会收集本次操作的信息
	//	fit.RedisClient(fit.WithGinTraceCtx(c)).Get("KKKK")
	//	c.String(http.StatusOK, "OK")
	//})

	g.Run(":8003")
}
