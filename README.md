## 常用库封装

> 已有轮子不再造

## install

```go
go get github.com/source-build/go-fit
```

## 使用

### 日志收集

> 非常简单，直接看代码

```go
package main

import (
	"fmt"
	"github.com/source-build/go-fit"
	"log"
)

type remoteLogHook struct {
}

func (r remoteLogHook) Before(body string) string {
	return body
}

func (r remoteLogHook) Error(err error) {
	fmt.Println("当发生错误时触发", err)
}

func main() {
	/* 配置 */
	//开启本地日志
	fit.SetLocalLogConfig(fit.LogEntity{
		LogPath:      "logs",      //日志文件存放的路径，默认为根目录下的logs
		FileName:     "diagnosis", //日志文件名称
		IsDefaultLog: true,        //默认日志，当直接调用Error、Info时会选择默认的
	}, fit.LogEntity{
		LogPath:  "logs",
		FileName: "track",
	}, fit.LogEntity{
		LogPath:  "logs",
		FileName: "mysql",
	})
	////设置堆栈错误信息长度(默认300)，错误信息key应为err
	fit.SetLogStackLength(100)
	//开启控制台输出
	fit.SetOutputToConsole(true)

	////开启远程日志，这里使用rabbitMQ并使用路由模式，消息会原样发送
	fit.SetMqURL("amqp://guest:guest@127.0.0.1:5672")
	fit.SetRemoteRabbitMQLog(&fit.RemoteRabbitMQLog{ //
		Exchange: "abnormalHandle", //交换机名称
		Key:      "error",          //key，与此名称绑定的队列才能消费消息
		Durable:  true,             //交换机持久化
	})

	//如果你想使用指定
	//参数1: 日志文件名称，也就是 开启本地日志 的FileName字段
	//参数2: 配置
	//	fit.UseConsole() 输出到控制台
	//	fit.UseLocal()   输出到本地文件
	//	fit.UseRemote()  输出到远程mq
	//  fit.UseReportCaller(true) 记录文件名，行数
	//  fit.UseSetSkip(2) 上溯的栈帧数,输出发生错误的位置，包括文件名和行数，参数为 栈帧数。fit.UseReportCaller(true) 时有效
	fit.OtherLog("track", fit.UseLocal()).Error("这是信息消息")

	//如果你只想写入本地而且不受全局配置的影响，可以使用以下方式，不过还是需要开启本地日志
	//如果有参数，则会使用指定的日志实例写入，需要在 开启本地日志
	fit.LocalLog("track").Info("error info")

	//如果你只想使用远程而且不受全局配置的影响，可以使用以下方式，不过还是需要开启远程支持
	// 第一个参数是日志类型，当远程写入失败时会将错误信息写入本地
	// 第二个参数跟 Error Warning Fatal 用法一致
	fit.RemoteLog(fit.ErrorLevel, "msg", "获取用户信息失败")

	//在远程日志发送之前做点什么?
	fit.AddRemoteLogHook(new(remoteLogHook))

	//自定义错误处理
	go func() {
		c := fit.CustomizeLog()
		defer fit.CloseCustomizeLog()
		for msg := range c {
			fmt.Println("错误信息：", msg)
		}
	}()

	//获取logrus实例
	fit.GetLogInstances()
	instance, ok := fit.GetLogInstance("mysql")
	if !ok {
		log.Fatalln("not find")
	}
	instance.Error()

	//快捷使用
	fit.Error()   //错误
	fit.Warning() //警告
	fit.Info()    //消息
	fit.Fatal()   //致命的

	fit.ErrorJSON(fit.H{"title": "666"})
	fit.WarningJSON(fit.H{"title": "666"})
	fit.InfoJSON(fit.H{"title": "666"})
	fit.FatalJSON(fit.H{"title": "666"})

	/* 其他用法 */
	s := fit.Fields{"key": "value"}.ToSlice()
	fit.Error(s...)
}
```

### 简单的链路追踪

> 直接上代码

#### gin使用

```go
package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/source-build/go-fit"
	"gorm.io/gorm"
	"log"
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
	err := fit.ConnectDefaultConfigMysql(fit.DefaultConfigMysql{
		User: "test",
		Pass: "123456",
		IP:   "127.0.0.1",
		Port: "3316",
		DB:   "user",
	}, true)
	if err != nil {
		log.Fatalln(err)
	}

	//连接redis单节点
	err = fit.NewRedisDefConnect("127.0.0.1:6380", "", "", 0)
	if err != nil {
		log.Fatalln(err)
	}
	defer fit.CloseRedis()

	g := gin.New()
	/* ====== 创建 ====== */
	//参数: 需要写入到的日志文件名称，需要预先配置好, 说白了就是上面的 FileName 字段
	//如果不传则则不写入本地日志
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
	g.Use(gt.TraceHandler)

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
		fit.NewMySQL().Set(fit.TraceCaller()).WithContext(c).Where("id = ?", 9).Take(&user)
		c.String(http.StatusOK, "OK")
	})

	/* 记录Redis信息 */
	g.GET("/redis", func(c *gin.Context) {
		//使用fit.WithGinTraceCtx(c)传递当前context,会收集本次操作的信息
		fit.RedisClient(fit.WithGinTraceCtx(c)).Get("KKKK")
		c.String(http.StatusOK, "OK")
	})

	/* 记录第三方请求信息 */
	g.GET("/thirdParty", func(c *gin.Context) {
		trace, _ := fit.GetGinTraceCtx(c)
		trace.AppendThirdPartyReq(&fit.LinkTraceDialog{
			Request:   nil,
			Responses: nil,
			Success:   false,
			Cost:      "",
		})
		c.String(http.StatusOK, "OK")
	})

	g.Run(":8003")
}
```

#### rpc使用

##### 服务端

```go
func main() {
/* 开启本地日志 */
fit.SetLocalLogConfig(fit.LogEntity{
LogPath:      "logs",  //修改日志路径，默认为根目录下的logs
FileName:     "track", //日志文件名称
Formatter:    fit.JSONFormatter, //格式化方式,不传默认json。可选text(fit.TextFormatter)|json(fit.JSONFormatter)
IsDefaultLog: true,
ReportCaller: true, //输出文件名 行数, IsDefaultLog = true 时生效
})

/* ====== 创建 ====== */
//参数: 需要写入到的日志文件名称，需要预先配置好, 说白了就是上面的 FileName 字段
//如果不传则不写入本地日志
gt := fit.NewLinkTrace("track")
//写入方式：LOCAL 本地 REMOTE 远程 CONSOLE 终端。NewGinTrace 有参数时才生效
gt.SetRecordMode("LOCAL")
//设置服务名称
gt.SetServiceName("user")
//设置服务类型，如api服务、rpc服务等
gt.SetServiceType("rpc")

var opts []grpc.ServerOption

//日志收集
//由于只能设置一个拦截器，如果你也想使用拦截器，则需要添加一个hook
//gt.GrpcHook(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
//	//如果不调用handler，将不会继续往下处理
//	fmt.Println("请求来了")
//	res, err := handler(ctx, req)
//	return res, err
//})
//注意：这是一元拦截器
opts = append(opts, grpc.UnaryInterceptor(gt.GrpcServerInterceptor()))

rpcServer := grpc.NewServer(opts...)
pb.RegisterPhoneLoginSmsVerCodeServer(rpcServer, new(phoneSms))

quit := make(chan os.Signal, 1)
go func () {
signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGKILL)
if err := rpcServer.Serve(listen); err != nil {
log.Fatalln(err)
}
}()
<-quit
fmt.Println("service close!")
}

type phoneSms struct {
pb.UnimplementedPhoneLoginSmsVerCodeServer
}

func (p phoneSms) Send(ctx context.Context, request *pb.SendRequest) (*pb.Response, error) {
//获取trace
trace, ok := fit.GetTraceCtx(ctx)
if ok {
fmt.Println(trace)
}
return &pb.Response{
Msg:    "OK",
Code:   0,
Result: "OK",
}, nil
}
```

##### 客户端

```go
func main() {
//连接etcd
client, err := clientv3.New(clientv3.Config{
Endpoints:   []string{"127.0.0.1:2479"},
DialTimeout: time.Second * 5,
})
if err != nil {
log.Fatalln(err)
}

/* ====== 创建 ====== */
//参数: 需要写入到的日志文件名称，需要预先配置好, 说白了就是上面的 FileName 字段
//如果不传则不写入本地日志
gt := fit.NewLinkTrace()
//写入方式：LOCAL 本地 REMOTE 远程 CONSOLE 终端。NewGinTrace 有参数时才生效
//gt.SetRecordMode("LOCAL")
//设置服务名称
gt.SetServiceName("user")
//设置服务类型，如api服务、rpc服务等
gt.SetServiceType("api")

//初始化客户端解析器
//发起grpc请求时会自动解析并使用负载均衡策略
err = fit.NewGrpcClientBuilder(fit.GrpcBuilderConfig{
EtcdClient:         client,
ClientCertPath:     "./keys/client.crt",
ClientKeyPath:      "./keys/client.key",
RootCrtPath:        "./keys/ca.crt",
ServerNameOverride: "SourceBuild.cn",
})
if err != nil {
log.Fatalln(err)
}

g := gin.New()
g.Use(gt.GinTraceHandler())

g.GET("/", func (c *gin.Context) {
//传递fit.WithContext()会在拦截器中记录操作信息，耗时等,
conn, err := fit.GrpcDial("/serves/rpc/dpp",
fit.Attempts(5),
fit.WithContext(),
)
if err != nil {
log.Fatalln(err)
}
defer conn.Close()

resp := pb.NewPhoneLoginSmsVerCodeClient(conn)
//记录rpc调用信息，需要传递context
res, err := resp.Send(c, &pb.SendRequest{
PhoneCode:  "OK",
Expired:    200,
TemplateId: 0,
})
if err != nil {
c.String(http.StatusOK, "ERR")
return
}

fmt.Println(res.Msg)

c.String(http.StatusOK, "OK")
})
g.Run(":8005")
}
```

#### 结果

```json
 {
  "trace_id": "d2252a9a-6995-4148-9f26-d7dd5f7c3f93",
  "request": {
    "method": "GET",
    "url": "/mysql",
    "header": {
      "Accept": [
        "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9"
      ],
      "Accept-Encoding": [
        "gzip, deflate, br"
      ],
      "Accept-Language": [
        "zh-CN,zh;q=0.9,en;q=0.8"
      ],
      "Cache-Control": [
        "max-age=0"
      ],
      "Connection": [
        "keep-alive"
      ],
      "Cookie": [
        "mobile-Token=eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJsb2dpbklkIjoic3lzX3VzZXI6MTEyNSIsInJuIjoiUzhFVnpNSXY5YkpYTGoyd2ZVOW1tdFhYOHdtUFJjcFMifQ.3Mw1UaOqGBEtAh0T_uTLnmC7mX9r0KlynzzhXmJR8eg; Admin-Token=eyJhbGciOiJIUzUxMiJ9.eyJsb2dpbl91c2VyX2tleSI6ImM2NTY0ZTRhLWEwNzgtNDkyYi04YjAxLWRlODVhZDFjY2QxNiJ9.3bbJdhVbtQ3wd5kEoacRoKayRqWYs36Lc0qi9Pv31JYI4tVAcXeGHzfhPdrOAmbbei6P15PXT_5NZb07w0Eguw; sidebarStatus=0"
      ],
      "Sec-Ch-Ua": [
        "\"Chromium\";v=\"104\", \" Not A;Brand\";v=\"99\", \"Google Chrome\";v=\"104\""
      ],
      "Sec-Ch-Ua-Mobile": [
        "?0"
      ],
      "Sec-Ch-Ua-Platform": [
        "\"macOS\""
      ],
      "Sec-Fetch-Dest": [
        "document"
      ],
      "Sec-Fetch-Mode": [
        "navigate"
      ],
      "Sec-Fetch-Site": [
        "none"
      ],
      "Sec-Fetch-User": [
        "?1"
      ],
      "Upgrade-Insecure-Requests": [
        "1"
      ],
      "User-Agent": [
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.0.0 Safari/537.36"
      ]
    },
    "body": {
    }
  },
  "response": {
    "header": {
      "Content-Type": [
        "text/plain; charset=utf-8"
      ]
    },
    "body": "OK",
    "http_code": 200,
    "http_msg": "",
    "cost": ""
  },
  "third_party_requests": null,
  "sqls": [
    {
      "timestamp": "2022-08-31 18:07:04",
      "stack": "main.go:87",
      "sql": "SELECT * FROM `users` WHERE id = 9 AND `users`.`deleted_at` IS NULL LIMIT 1",
      "rows_affected": 1,
      "cost": "94.746375ms"
    }
  ],
  "redis": null,
  "success": true,
  "start": 1661940424,
  "end": 1661940424,
  "cost": "94.942791ms",
  "extend": null
}
```

### 防止缓存击穿

> 引用库: golang.org/x/sync/singleflight

#### 示例代码

```go
package main

import "errors"

var gsf singleflight.Group

func main() {
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	//模拟100个并发
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(c context.Context) {
			defer wg.Done()
			data, err := getData(c, "key")
			if err != nil {
				log.Println("错误", err)
				return
			}
			log.Println(data)
		}(ctx)
	}
	wg.Wait()
}

//获取数据
func getData(ctx context.Context, key string) (string, error) {
	//模拟从缓存中获取数据
	data, err := getDataFromCache(key)
	if err != nil {
		//缓存中数据不存在，模拟从db中获取数据
		//使用超时控制
		v, err, _ := fit.NewSingle().DoChan(ctx, &gsf, key, func() (interface{}, error) {
			return getDataFromDB(key)
		})
		if err != nil {
			return "", err
		}
		data = v.(string)

		//使用同步方法
		//v, err, _ := gsf.Do(key, func() (interface{}, error) {
		//	return getDataFromDB(key)
		//})
		//if err != nil {
		//	return "", err
		//}
		//data = v.(string)
	}
	return data, nil
}

//模拟从cache中获取值，cache中无该值
func getDataFromCache(key string) (string, error) {
	return "", errors.New("err")
}

//模拟从数据库中获取值
func getDataFromDB(key string) (string, error) {
	log.Printf("get %s from database", key)
	return "data", nil
}
```

#### 所有方法

##### 同步阻塞

> 只有第一个请求会被执行getDataFromDB(key)，同一资源下的其余请求会阻塞等待
> 如果代码出问题,全员阻塞

```go
func main() {
var gsf singleflight.Group
//返回值:v 就是getDataFromDB返回的第一个参数、err 错误信息,这个应该都懂、shared 是否将v赋给了多个调用方
v, err, shared := gsf.Do(key, func () (interface{}, error) {
//getDataFromDB(key) //查询db
return getDataFromDB(key)
})
}
```

##### 异步返回

```go
func main() {
var gsf singleflight.Group
res := gsf.DoChan(key, func () (interface{}, error) {
return getDataFromDB(key)
})
//返回值 r.Val 就是getDataFromDB返回的第一个参数、r.Err 错误信息,这个应该都懂、r.Shared 是否将v赋给了多个调用方
r := <-res
if r.Err != nil {
log.Println(err)
}
data = r.Val.(string)
}
```

##### 异步返回|超时控制

> 假如一次调用要 1s，数据库请求或者是下游服务可以支撑10rps的时候这会导致错误阈提高。
> 我们可以一秒内尝试 10 次
> 像这样 fit.NewSingle(time.Millisecond*100)

```go
func main() {
var gsf singleflight.Group
//超时时间5秒
ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
defer cancel()

//返回值:v 就是getDataFromDB返回的第一个参数、err 错误信息,这个应该都懂、shared 是否将v赋给了多个调用方
v, err, shared := fit.NewSingle().DoChan(ctx, &gsf, key, func () (interface{}, error) {
return getDataFromDB(key)
})
}
```

### 请求重试

在微服务架构中，通常会有很多的小服务，小服务之间存在大量 RPC 调用，但时常因为网络抖动等原因，造成请求失败，
这时候使用重试机制可以提高请求的最终成功率，减少故障影响，让系统运行更稳定。retry-go 是一个功能比较完善的 golang 重试库。

> 使用灰常的简单，话不多说，上代码

```go
package main

import (
	"fmt"
	"github.com/avast/retry-go/v4"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

func main() {
	url := "http://example.com"
	var body []byte

	err := retry.Do(
		func() error {
			resp, err := http.Get(url)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			body, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			return nil
		},
		retry.Attempts(10), //最大重试次数
		//retry.Delay(time.Second*2), //重试延迟时间
		//retry.MaxDelay(time.Second*3), //最大重试延迟时间，选择指数退避策略时，该配置会限制等待时间上限
		//retry.MaxJitter(time.Second*10), //随机退避策略的最大等待时间
		//retry.OnRetry(func(n uint, err error) {}), //每次重试时会调用一次
		/*退避策略类型*/
		//BackOffDelay 退避策略
		//对于一些暂时性的错误，如网络抖动等，立即重试可能还是会失败，通常等待一小会儿再重试的话成功率会较高，
		//并且这种策略也可以打散上游重试的时间，避免同时重试而导致的瞬间流量高峰。
		//BackOffDelay 提供一个指数避退策略，连续重试时，每次等待时间都是前一次的 2 倍。
		//FixedDelay 在每次重试时，等待一个固定延迟时间。
		//RandomDelay 在 0 - config.maxJitter 内随机等待一个时间后重试。
		//CombineDelay  提供结合多种策略实现一个新策略的能力。
		retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
			fmt.Println("发生错误: " + err.Error())
			return retry.BackOffDelay(n, err, config)
		}),
		//retry.LastErrorOnly(false),//是否只返回上次重试的错误
	)

	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(body))
}
```

### 监控

生产者代码

```go
package main

import (
	"context"
	"github.com/source-build/go-fit"
	clientv3 "go.etcd.io/etcd/client/v3"
	"log"
	"time"
)

func main() {
	//连接redis单节点
	err := fit.NewRedisDefConnect("192.168.1.1:6380", "", "", 0)
	if err != nil {
		log.Fatalln(err)
	}
	defer fit.CloseRedis()

	err = fit.InitEtcd(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2479"},
		DialTimeout: time.Second * 10,
	})
	if err != nil {
		log.Fatalln(err)
	}

	fit.SetMqURL("amqp://guest:guest@192.168.1.1:5672")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	//使用
	err = fit.ServiceMonitorTask(&fit.ServiceMonitorOption{
		Context:               ctx,
		ServiceNode:           "ikkl",             //节点名称
		ServiceName:           "user",             //服务名称
		ServiceType:           "api",              //服务类型
		ServiceAddress:        "192.168.1.1:6004", //服务地址
		SystemVersion:         "1.0.1",            //系统版本
		RecordRedisClientInfo: true,               //是否返回redisClient
		RecordRedisStatsInfo:  true,               //是否返回redis统计信息
	})
	if err != nil {
		log.Fatalln(err)
	}
	select {}
}
```

消费端代码
>MQ
```go
//设置mq地址
	fit.SetMqURL("amqp://guest:guest@192.168.1.1:5672")
	//新建实例
	mq, err := fit.NewRabbitMQ()
	if err != nil {
		log.Fatal(err)
	}
	//释放资源,建议NewRabbitMQ获取实例后 配合defer使用
	defer mq.Close()

	//创建交换器
	ex := mq.DefExchangeDeclare("service_monitor", fit.KIND_DIRECT, false, true)
	//随机生成队列名
	msgs, err := ex.QueueDeclare("", false, true, false, false, nil).
		ReceiveRouting("monitor") //路由key
	if err != nil {
		log.Fatalln(err)
	}
	for msg := range msgs {
		fmt.Println("message:", string(msg.Body))
		//主动应答
		err := msg.Ack(true)
	}
```
>HTTP
```go
package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/source-build/go-fit"
	"net/http"
)

func main() {
	g := gin.New()
	g.POST("/msg", func(c *gin.Context) {
		var body fit.MessageBody
		err := c.ShouldBindJSON(&body)
		if err != nil {
			c.String(http.StatusBadRequest, "ERR")
			return
		}
		fmt.Printf("%+v\n", body)
	})
	g.Run(":8008")
}
```
etcd中的key格式示例
> api/user/ikkl 加上后面的节点名称（ikkl）用于指定那个服务采集机器负载信息
etcd中的value配置示例
```json
{  
  stage: "INIT", //阶段，可选值 INIT、WORK
  //当etcd服务终止或找不到etcd存活时，将自动退出任务，如果为false，则会阻塞一直等到etcd服务恢复后继续执行任务。 
  downtimeAutoQuit:true, 
  returnWorkTask: true, //是否返回当前工作的协程数量
  returnMem: true, //是否返回内存信息
  returnCpu: true, //是否返回CPU信息
  returnIoCount: true, //是否获取网络读写字节／包的个数
  subType: "", //接收类型 HTTP、MQ
  subHttpUrl: "", //http url，默认post方式，subType = HTTP生效
  subHttpToken: "", //http 请求时需要携带的token，如果subHttpHeader存在,则该字段会被覆盖,subType = HTTP生效
  subHttpHeader: "", //subType = HTTP生效
  mqWorkType: "", //simple 简单模式、 work 工作模式、 publish 发布订阅模式 routing 模式
  mqDeclareName: "", //声明时的队列名称，为空则随机生成
  mqDeclareDurable: false, //队列是否需要持久化，不持久化重启mq将失效。
  mqAutoDelete: false, //自动删除？
  mqExchangeName: "", //声明时的交换机名称，注意：simple、work模式时不需要填
  mqExchangeDurable: false, //交换机是否需要持久化，不持久化重启mq将失效。
  // 当mqWorkType=routing时，需要设置此字段接收时才会与路由精确匹配上，
  //如果为空则默认路由名称为 monitor。
  mqRoutingKey:"", 
  duration:3, //多久发送一次，默认5s，单位s
  //最大重试连接次数，当etcd服务不可用时，会进行重试.
  //注意，这里重试指的是etcd。
  retryCount:5,
};
```
>注意:
如果使用http的方式接收，响应状态码!=200时，会重试请求最多三次！
INIT：初始状态、 WORK：工作状态
首次应为INIT，INIT阶段return*字段不生效，也就是说，stage=INIT时，不需要return*开头的字段，随后服务监听接收到该值后，假设你选择接收类型为mq，那么会向mq发送一条包含服务所在的机器信息，这样就能拿到服务所在的机器唯一id，最后你再确定由哪一台机器负责采集负载信息。一些情况下同一台机器中会部署多个服务集群等，如果每个服务都要采集机器信息，这是没有必要的，因为他们都在同一台机器上。

### rabbitMQ

#### 基本配置

```go
func main() {
//设置mq地址	
fit.SetMqURL("amqp://guest:guest@192.168.66.13:5672")
//新建实例	
mq, err := fit.NewRabbitMQ()
if err != nil {
log.Fatal(err)
}
//释放资源,建议NewRabbitMQ获取实例后 配合defer使用
defer mq.Close()
//获取conn
//mq.Conn()

//获取channel，所有操作都是基于channel
//mq.Channel()		

//(全局生效)设置错误处理方式（默认写入本地日志，不过也需配置本地日志才生效）
//可传多个 可选值:
//	- ALL 根据日志配置以所有的方式写入
//  - LOCAL 仅写入本地日志（需配置）
//  - REMOTE 仅写入远程日志（需配置）
//  - CONSOLE 仅将错误输出到控制台
fit.SetRabbitMqErrLogHandle(fit.ALL)
//当前实例生效(优先级比全局配置高)
mq.SetRabbitMqErrLogHandle(fit.ALL)
}
```

#### 声明队列

```go
func main() {
//使用默认声明队列。参数说明: name 队列名称 durable 是否持久化 autoDelete 是否自动删除 
mq.DefQueueDeclare(name, durable, autoDelete)
//声明队列。跟官方的参数一致，有点多，自己点进去看😊
mq.QueueDeclare()
//注意：name 为空则随机生成
//小贴士: 声明队列支持链式调用,像这样：mq.DefQueueDeclare("logs", false,false).PublishSimple()
}
```

#### 声明交换机

```go
func main() {
//默认交换机。参数 name:名称 kind:交换器模式｜可选值 fit.KIND_* durable 是否持久化	
mq.DefExchangeDeclare(name, kind, durable,autoDel)
//跟官方的参数一致，有点多，自己点进去看😊
mq.ExchangeDeclare()
// 小贴士: 同样支持链式调用,像这样：mq.DefExchangeDeclare().PublishPub()
}
```

#### simple（简单模式）

注意️：简单模式(最简单的收发模式)中，不需要用到交换机，所以复制粘贴食用，

```go
func main() {
mq, _ := fit.NewRabbitMQ()
//-------------------- 生产者 --------------------
err = mq.DefQueueDeclare("logs", false,false).PublishSimple("这是内容")
if err != nil {
fmt.Println(err)
}
fmt.Println("发送成功！")

//-------------------- 消费者 --------------------
// mq.ConsumeSimple() 使用默认配置创建消费者
// mq.ConsumeSimple(fit.ConsumeConfig{}) 完整配置创建消费者
simple, err := mq.DefQueueDeclare("logs", false, true).ConsumeSimple()
if err != nil {
log.Fatal(err)
}
for msg := range simple {
fmt.Println(string(msg.Body))
//主动应答
//如果autoAck字段为false(默认)，则需要手动调用msg.Ack(),否则会造成内存溢出
//如果autoAck字段为true,则服务器将自动确认每条消息，并且不应调用此方法
if err := msg.Ack(true); err != nil {
log.Fatal("主动应答失败:", err)
}
}
}
```

#### work（工作模式）

消费者多个的情况下消息会以轮询的方式公平分发，每个消费者消费的次数相同。

```go
func main() {
mq, _ := fit.NewRabbitMQ()
//-------------------- 生产者 --------------------
err = mq.DefQueueDeclare("logs", false,false).PublishSimple("这是内容")
if err != nil {
fmt.Println(err)
}
fmt.Println("发送成功！")

//-------------------- 消费者 --------------------
// mq.ConsumeSimple() 使用默认配置创建消费者
// mq.ConsumeSimple(fit.ConsumeConfig{}) 完整配置创建消费者
simple, err := mq.DefQueueDeclare("logs", false, true).ConsumeSimple()
if err != nil {
log.Fatal(err)
}
for msg := range simple {
fmt.Println(string(msg.Body))
//主动应答
//如果autoAck字段为false(默认)，则需要手动调用msg.Ack(),否则会造成内存溢出
//如果autoAck字段为true,则服务器将自动确认每条消息，并且不应调用此方法
if err := msg.Ack(true); err != nil {
log.Fatal("主动应答失败:", err)
}
}
}
```

#### publish/subscribe（发布订阅模式）

话不多说，这里我就当大家都知道发布订阅模式了
生产者发消息broker，由交换机将消息转发到绑定此交换机的每个队列，每个绑定交换机的队列都将接收到消息。

```go
func main() {
mq, _ := fit.NewRabbitMQ()
//-------------------- 生产者(发布) --------------------
//声明交换机，fit.KIND_FANOUT 表示广播到所有与此绑定的队列
err = mq.DefExchangeDeclare("exchange_test1", fit.KIND_FANOUT, false, false).PublishPub("这是新的消息") //将消息发送到 exchange_test1 交换机上
if err != nil {
log.Fatal(err)
}
fmt.Println("发布成功")

//-------------------- 消费者(订阅) --------------------
//ReceiveSub()方法参数为空则使用默认配置的消费者
msgs, err := mq.DefQueueDeclare("", false,false).
DefExchangeDeclare("exchange_test1", fit.KIND_FANOUT, false, false).ReceiveSub()
if err != nil {
log.Fatal(err)
}
for msg := range msgs {
fmt.Println(string(msg.Body))
}
}
```

#### routing（路由模式）

消息生产者将消息发送给交换机按照路由判断,路由是字符串(info) 当前产生的消息携带路由字符(对象的方法),
交换机根据路由的key,只能匹配上路由key对应的消息队列

```go
func main() {
mq, _ := fit.NewRabbitMQ()
//-------------------- 生产者 --------------------
//声明交换机。fit.KIND_DIRECT 交换机将会对binding key和routing key进行精确匹配，从而确定消息该分发到哪个队列
mq = mq.DefExchangeDeclare("exchange_test2", fit.KIND_DIRECT, true, false)
//将消息发送到 exchange_test2 交换机上
if err := mq.Publish("这是新的消息", "error"); err != nil {
log.Fatal(err)
}
fmt.Println("发布成功")

//-------------------- 消费者 --------------------
//创建交换机
ex := mq.DefExchangeDeclare("exchange_test2", fit.KIND_DIRECT, true, false)
//随机生成队列名
msgs, err = ex.QueueDeclare("", false, false, true, false, nil).
ReceiveRouting("error") //路由key
if err != nil {
log.Fatalln(err)
}
for msg := range msgs {
fmt.Println("来消息了", string(msg.Body))
//主动应答
if err := msg.Ack(true); err != nil {
log.Fatal("主动应答失败:", err)
}
}
}
```

#### topic（主题模式）

交换机根据key的规则模糊匹配到对应的队列,由队列的监听消费者接收消息消费

- 星号井号代表通配符
- 星号代表多个单词,井号代表一个单词
- 路由功能添加模糊匹配

```go
func main() {
mq, _ := fit.NewRabbitMQ()
//-------------------- 生产者 --------------------
//声明交换机。fit.KIND_DIRECT 交换机将会对binding key和routing key进行精确匹配，从而确定消息该分发到哪个队列
mq = mq.DefExchangeDeclare("exchange_test3", fit.KIND_TOPIC, true, false)
//将消息发送到 exchange_test3 交换机上,注意通配符说明
//如：hello.* == hello.world | 匹配多个单词: hello.# == hello.world.one
if err := mq.PublishTopic("这是新的消息6666", "hello.*"); err != nil {
log.Fatal(err)
}
fmt.Println("发布成功")

//-------------------- 消费者 --------------------
//创建交换机
ex := mq.DefExchangeDeclare("exchange_test2", fit.KIND_TOPIC, true, false)
//随机生成队列名
msgs, err := ex.QueueDeclare("", false, false, true, false, nil).
ReceiveTopic("hello.world")
if err != nil {
log.Fatalln(err)
}

for msg := range msgs {
fmt.Println("来消息了", string(msg.Body))
//主动应答
err := msg.Ack(true)
if err != nil {
log.Fatal("主动应答失败:", err)
}
}
}
```

#### 自定义

以上只提供了对我而言比较方便的用法，如果不满足你的需求，那就自己调用 **mq.Channel()**

### gRPC

#### 客户端

```go
func main() {
// ClientInterceptor 客户端拦截器
func ClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
start := time.Now()
err := invoker(ctx, method, req, reply, cc, opts...)
log.Printf("method == %s ; req == %v ; rep == %v ; duration == %s ; error == %v\n", method, req, reply, time.Since(start), err)
return err
}
func main() {
//连接etcd
client, err := clientv3.New(clientv3.Config{
Endpoints:   []string{"127.0.0.1:2379"},
DialTimeout: time.Second * 5,
})
if err != nil {
log.Fatalln(err)
}

err = fit.InitSentinel(fit.SentinelConfig{
Version: "1.0.0",
AppName: "userApi",
LogDir:  "", //日志文件位置，秒级日志，为空则不输出日志
})
if err != nil {
log.Fatalln(err)
}

// https://sentinelguard.io/zh-cn/docs/golang/circuit-breaking.html
breakerRules := []*circuitbreaker.Rule{
// 错误比例规则,统计周期内资源请求访问异常的比例大于设定的阈值，则接下来的熔断周期内对资源的访问会自动地被熔断
{
Resource:         "errorRatio",
Strategy:         circuitbreaker.ErrorRatio,
RetryTimeoutMs:   3000, //熔断触发后持续的时间（单位为 ms）
MinRequestAmount: 10,   //静默请求数
StatIntervalMs:   5000, //统计周期
Threshold:        0.4, //错误比例的阈值(小数表示，比如0.1表示10%)
},
}
//加载熔断规则
if err := fit.LoadBreakerRule(breakerRules); err != nil {
log.Fatalln(err)
}

//初始化客户端解析器
//发起grpc请求时会自动解析并使用负载均衡策略
err = fit.NewGrpcClientBuilder(fit.GrpcBuilderConfig{
EtcdClient:         client,
ClientCertPath:     "./keys/client.crt",
ClientKeyPath:      "./keys/client.key",
RootCrtPath:        "./keys/ca.crt",
ServerNameOverride: "SourceBuild.cn",
})
if err != nil {
log.Fatalln(err)
}

//参数一:服务名称，下面会讲
//参数二:配置
//fit.Attempts(5) //使用重试策略，参数为重试次数
//fit.Rule("errorRatio") //使用熔断机制，与重试策略而二选一
//fit.DialOption() //添加gRPC配置，具体可以看看gRPC文档
//注意：服务名称，也就是etcd的key
// 如果etcd中有多个服务注册,比如有这些key: /serves/rpc/user/1 /serves/rpc/user/2 /serves/rpc/user/3
// 那么调用GrpcDial时只需要写 /serves/rpc/user 这样就会以轮训的方式依次请求 1 2 3
conn, err := fit.GrpcDial("/serves/rpc/user",
fit.Attempts(5),
fit.DialOption(grpc.WithUnaryInterceptor(ClientInterceptor)), //添加客户端拦截器
)
if err != nil {
log.Fatalln("连接失败", err)
}
defer conn.Close()

e := pb.NewEmailServiceClient(conn)
res, err := e.SendEmailVerificationCode(nil, &pb.VerCodeRequest{
Email:  "123@qq.com",
Expire: 66,
})
if err != nil {
log.Fatalln(err)
}

fmt.Println(res.Code, res.Msg)
}
}
```

### 服务注册与发现

#### 服务注册

服务启动时将服务注册到etcd中，可以通过etcd监控与发现服务

```go
client, err := clientv3.New(clientv3.Config{
Endpoints:   []string{"127.0.0.1:2379"},
DialTimeout: time.Second * 5,
})
//获取ip
localIp, _ := fit.GetOutBoundIP()

//服务注册
s, err := fit.NewServiceRegister(&fit.ServiceRegister{
Ctx:    context.Background(),
Client: client,
Key:    "/serves/rpc/dpp/Mjhd",
Value:  localIp + ":8080",
Lease:  20,
})

if err != nil {
log.Fatalln(err)
}

fmt.Println("service start success!!!")

quit := make(chan os.Signal, 1)
go func () {
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
}()
<-quit
s.Close()
fmt.Println("service close!")
```

#### 服务发现

```go
//连接etcd
client, err := clientv3.New(clientv3.Config{
Endpoints:   []string{"127.0.0.1:2379"},
DialTimeout: time.Second * 5,
})
if err != nil {
log.Fatalln(err)
}
//服务发现
result, err := fit.NewServiceDiscovery(context.Background(), client, "/services/rpc/messagepush/")
if err != nil {
log.Fatalln(err)
}
fmt.Println(result.SelectByRand()) //随机获取一个服务地址
```

### 身份验证

#### Token

```go
key := "lpl654"
//生成token
jwtClaims := fit.JwtClaims{
ExpiresAt: time.Now().Add(time.Minute).Unix(),
Id:        "45565",
Subject:   "user_login",
}
str, err := fit.NewJwtClaims(key, jwtClaims)
if err != nil {
log.Fatalln(str)
}
fmt.Println(str)

//验证token
t, err := fit.Valid(key, str)
if err != nil {
log.Fatalln(err)
}
fmt.Println("success")
fmt.Printf("%+v", t)
```

#### TTL

### 流量控制

流量控制(flow control)，其原理是监控资源(Resource)的统计指标，然后根据 token 计算策略来计算资源的可用 token(也就是阈值)
，然后根据流量控制策略对请求进行控制，避免被瞬时的流量高峰冲垮，从而保障应用的高可用性。

#### 示例:内存自适应

```go
err := fit.InitSentinel(fit.SentinelConfig{
Version: "1.0.1",
AppName: "cs",
LogDir:  "",
})
if err != nil {
log.Fatalln(err)
}

flowRules = []*flow.Rule{
{
Resource:               "some-test1",
TokenCalculateStrategy: flow.MemoryAdaptive,
ControlBehavior:        flow.Reject, //Reject表示超过阈值直接拒绝
StatIntervalInMs:       1000,        // 规则对应的流量控制器的独立统计结构的统计周期。如果StatIntervalInMs是1000，也就是统计QPS。
LowMemUsageThreshold:   1000,
HighMemUsageThreshold:  100,
// 如果当前内存使用量为(MemLowWaterMarkBytes,MemHighWaterMarkBytes)
// 则阈值为（HighMemUsageThreshold，LowMemUsageThreshold）
MemLowWaterMarkBytes:  1024, // 如果当前内存使用量小于或等于MemLowWaterMarkBytes，则阈值(threshold)==LowMemUsageThreshold
MemHighWaterMarkBytes: 2048, // 如果当前内存使用量大于或等于MemHighWaterMarkBytes，则阈值(threshold)==HighMemUsageThreshold
},
}

//加载流控规则
err = fit.LoadFlowRule(flowRules)
if err != nil {
log.Fatalln(err)
}

// 模拟内存使用量为1000字节，因此QPS阈值应为1000
fmt.Println("内存使用量为999:", new(fit.ParseTime).HSM(time.Now().Unix()))
system_metric.SetSystemMemoryUsage(999)
ch := make(chan bool)
for i := 0; i < 10; i++ {
go func () {
for {
e, b := sentinel.Entry("some-test1", sentinel.WithTrafficType(base.Inbound))
if b != nil {
//已阻止。我们可以从BlockError中获取阻塞原因
time.Sleep(time.Duration(rand.Uint64()%2) * time.Millisecond)
} else {
// 通过
time.Sleep(time.Duration(rand.Uint64()%2) * time.Millisecond)
e.Exit()
}
}
}()
}

go func () {
time.Sleep(time.Second * 5)
// 模拟内存使用量为1536字节，因此QPS阈值应为550
system_metric.SetSystemMemoryUsage(1536)
fmt.Println("内存使用量为1536:", new(fit.ParseTime).HSM(time.Now().Unix()))

time.Sleep(time.Second * 5)
// 模拟内存使用量为1536字节，因此QPS阈值应为100
system_metric.SetSystemMemoryUsage(2048)
fmt.Println("内存使用量为2048:", new(fit.ParseTime).HSM(time.Now().Unix()))

time.Sleep(time.Second * 5)
// mock memory usage is 1536 bytes, so QPS threshold should be 100
system_metric.SetSystemMemoryUsage(100000)
fmt.Println("内存使用量为100000:", new(fit.ParseTime).HSM(time.Now().Unix()))
time.Sleep(time.Second * 5)
ch <- true
}()

t := time.NewTimer(time.Second * 5)
for {
select {
case <-t.C:
return
default:
}
e, b := sentinel.Entry("some-test")
if b != nil {
fmt.Println("禁止访问")
// 请求被拒绝，在此处进行处理
time.Sleep(time.Duration(rand.Uint64()%10) * time.Millisecond)
} else {
// 请求允许通过，此处编写业务逻辑
fmt.Println("允许访问")
time.Sleep(time.Duration(rand.Uint64()%10) * time.Millisecond)
// 务必保证业务结束后调用 Exit
e.Exit()
}
}
<-ch
```

#### 示例:qps 控制

以QPS=10为例

```go

func main() {
err := fit.InitSentinel(fit.SentinelConfig{
Version: "1.0.1",
AppName: "cs",
LogDir:  "./logs", //开启日志记录,秒级日志
})
if err != nil {
log.Fatalln(err)
}

flowRules := []*flow.Rule{
{
Resource:               "some-test",
Threshold:              10, //流控阈值；如果字段 StatIntervalInMs 是1000(也就是1秒)，那么Threshold就表示QPS，流量控制器也就会依据资源的QPS来做流控
TokenCalculateStrategy: flow.Direct,
ControlBehavior:        flow.Reject, //表示流量控制器的控制策略；Reject表示超过阈值直接拒绝，Throttling表示匀速排队
StatIntervalInMs:       1000, //规则对应的流量控制器的独立统计结构的统计周期。如果StatIntervalInMs是1000，也就是统计QPS。
},
}

err = fit.LoadFlowRule(flowRules)
if err != nil {
log.Fatalln(err)
}

//5秒后结束程序
t := time.NewTimer(time.Second * 5)
for {
select {
case <-t.C:
return
default:
}
e, b := sentinel.Entry("some-test")
if b != nil {
fmt.Println("禁止访问")
// 请求被拒绝，在此处进行处理
time.Sleep(time.Duration(rand.Uint64()%10) * time.Millisecond)
} else {
// 请求允许通过，此处编写业务逻辑
fmt.Println("允许访问")
time.Sleep(time.Duration(rand.Uint64()%10) * time.Millisecond)
// 务必保证业务结束后调用 Exit
e.Exit()
}
}
<-ch

}
```

#### 日志记录

该日志为qps控制示例的日志记录

```text
1655196924000|2022-06-14 16:55:24|some-test|10|114|10|0|5|0|1|0
1655196925000|2022-06-14 16:55:25|some-test|10|181|10|0|5|0|1|0
1655196926000|2022-06-14 16:55:26|some-test|10|172|10|0|5|0|1|0
1655196927000|2022-06-14 16:55:27|some-test|10|186|10|0|4|0|1|0
1655196928000|2022-06-14 16:55:28|some-test|10|187|10|0|3|0|1|0

#以上各字段含义分别为：
1. 时间戳
2. 日期
3. 资源名称
4. 这一秒通过的资源请求个数 (pass)
5. 这一秒资源被拦截的个数 (block)
6. 这一秒完成调用的资源个数 (complete)，包括正常结束和异常结束的情况
7. 这一秒资源的异常个数 (error)
8. 资源平均响应时间（ms）
```

### 熔断降级

在高可用设计中，除了流控外，对分布式系统调用链路中不稳定的资源(比如RPC服务等)进行熔断降级也是保障高可用的重要措施之一。现在的分布式架构中一个服务常常会调用第三方服务，这个第三方服务可能是另外的一个RPC接口、数据库，或者第三方 API
等等。例如，支付的时候，可能需要远程调用银联提供的
API；查询某个商品的价格，可能需要进行数据库查询。然而，除了自身服务外，依赖的外部服务的稳定性是不能绝对保证的。如果依赖的第三方服务出现了不稳定的情况，比如请求的响应时间变长，那么服务自身调用第三方服务的响应时间也会响应变长，也就是级联效应，服务自身的线程可能会产生堆积，最终可能耗尽业务自身的线程池，最终服务本身也变得不可用。

```go
var breakerRules = []*circuitbreaker.Rule{
// 慢调用比例规则
{
Resource:         "abc",
Strategy:         circuitbreaker.SlowRequestRatio, //慢调用比例策略。熔断策略，目前支持SlowRequestRatio、ErrorRatio、ErrorCount三种；
RetryTimeoutMs:   3000,                            //熔断触发后持续的时间（单位为 ms）。资源进入熔断状态后，在配置的熔断时长内，请求都会快速失败。熔断结束后进入探测恢复模式（HALF-OPEN）
MinRequestAmount: 10,                              //静默数量，若当前统计周期内的请求数小于此值，即使达到熔断条件规则也不会触发。
StatIntervalMs:   5000, //统计的时间窗口长度（单位为 ms）
MaxAllowedRtMs:   50,   //仅对慢调用熔断策略生效，MaxAllowedRtMs 是判断请求是否是慢调用的临界值，也就是如果请求的response time小于或等于MaxAllowedRtMs，那么就不是慢调用；如果response time大于MaxAllowedRtMs，那么当前请求就属于慢调用。
Threshold:        0.5, //对于错误比例策略，Threshold表示的是错误比例的阈值(小数表示，比如0.1表示10%)。
},
// 错误比例规则,统计周期内资源请求访问异常的比例大于设定的阈值，则接下来的熔断周期内对资源的访问会自动地被熔断
{
Resource:         "errorRatio",
Strategy:         circuitbreaker.ErrorRatio,
RetryTimeoutMs:   3000, //熔断触发后持续的时间（单位为 ms）
MinRequestAmount: 10,   //静默请求数
StatIntervalMs:   5000, //统计周期
Threshold:        0.4, //错误比例的阈值(小数表示，比如0.1表示10%)
},
}

type BreakerStatus struct {
}

// OnTransformToClosed 熔断器切换到 Closed 状态时候会调用改函数, prev代表切换前的状态，rule表示当前熔断器对应的规则
func (b BreakerStatus) OnTransformToClosed(prev circuitbreaker.State, rule circuitbreaker.Rule) {
fmt.Println("初始状态，该状态下，熔断器会保持闭合，对资源的访问直接通过熔断器的检查。")
}

// OnTransformToOpen 熔断器切换到 Open 状态时候会调用改函数, prev代表切换前的状态，rule表示当前熔断器对应的规则， snapshot表示触发熔断的值
func (b BreakerStatus) OnTransformToOpen(prev circuitbreaker.State, rule circuitbreaker.Rule, snapshot interface{}) {
fmt.Println("断开状态，熔断器处于开启状态，对资源的访问会被切断。")
}

// OnTransformToHalfOpen 熔断器切换到 HalfOpen 状态时候会调用改函数, prev代表切换前的状态，rule表示当前熔断器对应的规则
func (b BreakerStatus) OnTransformToHalfOpen(prev circuitbreaker.State, rule circuitbreaker.Rule) {
fmt.Println("半开状态，该状态下除了探测流量，其余对资源的访问也会被切断。")
}

func main() {
err := fit.InitSentinel(fit.SentinelConfig{
Version: "1.0.1",
AppName: "cs",
})
if err != nil {
log.Fatalln(err)
}
// 加载规则
err = fit.LoadBreakerRule(breakerRules, &BreakerStatus{})
if err != nil {
log.Fatalln(err)
}

conf := config.NewDefaultConfig()
conf.Sentinel.Log.Logger = logging.NewConsoleLogger()
conf.Sentinel.Stat.System.CollectIntervalMs = 0
conf.Sentinel.Stat.System.CollectMemoryIntervalMs = 0

//error_ratio
go func () {
for {
e, b := sentinel.Entry("errorRatio")
if b != nil {
//fmt.Println("g1 失败")
} else {
if rand.Uint64()%20 > 6 {
sentinel.TraceError(e, errors.New("biz error"))
fmt.Println("g1 错误上报")
}
fmt.Println("g1 成功")
e.Exit()
}
}
}()
//slow_request_ratio
go func () {
for {
e, b := sentinel.Entry("abc")
if b != nil {
fmt.Println("g2 失败")
return
} else {
if rand.Uint64()%20 > 6 {
sentinel.TraceError(e, errors.New("biz error"))
}
time.Sleep(time.Duration(rand.Uint64()%80+10) * time.Millisecond)
fmt.Println("g2 成功")
e.Exit()
}
}
}()
}
```

### redis

```go
package main

import (
	"github.com/source-build/go-fit"
	"log"
)

func main() {
	//连接redis单节点
	err := fit.NewRedisDefConnect("127.0.0.1:6379", "", "", 0)
	if err != nil {
		log.Fatalln(err)
	}
	defer fit.CloseRedis()

	////连接redis单节点，自定义配置
	//err = fit.NewRedisConnect(redis.Options{
	//	Addr:               "",
	//	Username:           "",
	//	Password:           "",
	//	DB:                 0,
	//	MinIdleConns:       0,
	//	MaxConnAge:         0,
	//	PoolTimeout:        0,
	//	IdleTimeout:        0,
	//	IdleCheckFrequency: 0,
	//	TLSConfig:          nil,
	//	Limiter:            nil,
	//})
	//if err != nil {
	//	log.Fatalln(err)
	//}
	//defer fit.CloseRedis()
	//
	////连接redis集群，默认0db
	//err = fit.NewRedisDefConnectCluster([]string{"127.0.0.1:6379", "127.0.0.1:6379"}, "", "")
	//
	////连接redis集群，自定义配置
	//err = fit.NewRedisConnectCluster(redis.ClusterOptions{
	//	Addrs:              nil,
	//	NewClient:          nil,
	//	MaxRedirects:       0,
	//	ReadOnly:           false,
	//	RouteByLatency:     false,
	//	RouteRandomly:      false,
	//	ClusterSlots:       nil,
	//	Dialer:             nil,
	//	OnConnect:          nil,
	//	Username:           "",
	//	Password:           "",
	//	MaxRetries:         0,
	//	MinRetryBackoff:    0,
	//	MaxRetryBackoff:    0,
	//	DialTimeout:        0,
	//	ReadTimeout:        0,
	//	WriteTimeout:       0,
	//	PoolFIFO:           false,
	//	PoolSize:           0,
	//	MinIdleConns:       0,
	//	MaxConnAge:         0,
	//	PoolTimeout:        0,
	//	IdleTimeout:        0,
	//	IdleCheckFrequency: 0,
	//	TLSConfig:          nil,
	//})

	/**
	 * 连接redis方式任意选一种就行，否则优先使用单节点
	 */

	/**
	  参数：可选
	  fit.CtxTimeout() 设置超时时间，默认10s
	  fit.DisableTimeout() 禁用超时时间
		fit.WithCtx() 传递context，不传 默认使用context.Background()
	  fit.WithGinTraceCtx() 传递gin.context,用于日志收集
		fit.WithExpire() 设置key过期时间，默认不过期
	*/
	instance := fit.MainRedis()
	//添加hook,GetClient() 获取单节点实例，GetCluster() 获取集群实例，取决于你初始化时用单节点连接还是集群连接
	//instance.GetCluster().AddHook()
	//获取单节点实例，连接单节点后使用
	instance.GetNode()
	//获取集群实例，连接集群后使用
	instance.GetCluster()
	//使用，如果你连接单节点，则会使用单节点实例，反之，集群也是同样的；
	_, err = instance.Set("key", "value")
	if err != nil {
		log.Fatalln(err)
	}

}
```

### mysql

```go
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
	fit.MainMysql().Create()
}
```

### etcd

```go
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
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second * 5,
	})
	if err != nil {
		log.Fatalln(err)
	}

	//使用
	res, err := fit.MainEtcdv3().Get("foo")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(res)
}
```

### 时间操作

```go
//获取此刻到明日凌晨00：00的时间差
t := fit.BeforeDawnTimeDifference()

//当前是否超过了给定时间
t := fit.SpecifiedTimeExceeded()

//获取完整时间
t := fit.GetFullTime(time.Now().Unix())
fmt.Println(t) //2022-06-14 21:51:04

t := fit.GetHMS(time.Now().Unix())
fmt.Println(t) //21:51:55

t := fit.GetMS(time.Now().Unix())
fmt.Println(t) //21:52
...
```

### 配置文件

#### 基础使用

```go
func init() {
flag.Int("service.port", 5002, "service port cannot be empty")
}

func main() {
//加载配置文件，支持yaml、json、ini等文件
//isUseParam: 是否支持命令行参数,默认false
err := fit.NewReadInConfig("./config.yaml", true)
if err != nil {
return
}
//使用
fmt.Println(viper.Get("service.port")) //5002
}
```

#### 动态配置

...

### 常用加密库

#### 密码加密

```go
//加密
pwd, err := fit.PasswordHash("123456")
if err != nil {
log.Fatalln(err)
}

//验证
if ok := fit.PasswordVerify("123456", pwd); !ok {
log.Fatalln("验证失败")
}
log.Println("验证成功")
```

#### MD5加密

```go
pwd := fit.MD5encryption("123456")
fmt.Println(pwd)
```

### 常用转换函数

#### Map转换为string(json)

```go
str := fit.H{"name": "张三", "sex": "男"}.ToString()
fmt.Println(str)
```

### 随机字符库

```go
//随机生成6位纯数字
fit.NewRandom().PureDigital(6)
//随机生成6位字母+纯数字
fit.NewRandom().LetterAndNumber(6)
//随机生成6位字母
fit.NewRandom().Char(6)
//随机生成6位字母字母+数字+ASCII字符
fit.NewRandom().CharAndNumberAscii(6)
```

### 转换库

#### struct 转 map

```go
type test struct {
Name string `json:"name"`
Age  int    `json:"age"`
Sex  int    `map:"sex"`
}

func main() {
testStruct := test{
Name: "张三",
Age:  19,
Sex:  1,
}
//第二个参数是要转换的字段对应的标签	
m := fit.StructConvertMapByTag(testStruct, "json")
fmt.Printf("%+v", m) //map[age:19 name:张三]
}
```

#### map转struct

```go
type user struct {
Name string `json:"name"`
Age  int    `json:"age"`
Sex  int    `map:"sex"`
}

func main() {
val := map[string]interface{}{
"name": "张三",
"age":  50,
"sex":  50,
}

var output user
if err := fit.MapConvertStruct(val, &output); err != nil {
return
}
fmt.Printf("%+v", output) //{Name:张三 Age:50 Sex:50}
}
```

#### struct 转 slice

```go
type test struct {
Name string `json:"name"`
Age  int    `json:"age"`
Sex  int    `map:"sex"`
}

func main() {
testStruct := test{
Name: "张三",
Age:  19,
Sex:  1,
}

s := fit.StructConvertSlice(testStruct, "json")
fmt.Printf("%+v", s) //[age 19 name 张三]
}
```

#### map转slice

```go
val := map[string]interface{}{
"name": "张三",
"age":  50,
"sex":  1,
}
fmt.Println(fit.MapConvertSlice(val)) //[name 张三 age 50 sex 1]
```
