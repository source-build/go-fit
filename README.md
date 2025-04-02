# Go开发工具包

> 已有轮子不再造

封装了开发中常用到的日志库、服务注册与发现、统一http响应体、快捷访问MySQL与Redis、请求参数校验、字符串操作、网络、随机数、加密、配置文件、时间、金额/小数操作等。

> 安装
> ```shell
> go get -u github.com/source-build/go-fit
> ```

## 日志库

> 基于 [zap库](https://markdown.com.cn) 封装
>
> zap 版本：v1.27.0

**特点**

- ✅ 日志切割、日志轮转
- ✅ 全局日志
- ✅ 开箱即用，直接调用包级别的函数输出日志

**初始化**

```go
opt := flog.Options{
// 日志等级 默认 info
LogLevel: flog.InfoLevel,
// 日志输出格式编码器，如果为Nil
EncoderConfigType: flog.ProductionEncoderConfig,
// 控制台输出
Console: true,
// EncoderConfigType 为 Nil时，可传此参数进行自定义 EncoderConfig。 
EncoderConfig: zapcore.EncoderConfig{},

// ------ 按大小轮转配置 ------
// 输出到文件，为空无效
Filename:   "logs/logger.log",
// 日志文件最大大小(MB)
MaxSize:    0,
// 保留旧日志文件的最大天数
MaxAge:     0,
// 保留日志文件的最大数量
MaxBackups: 0,
// 是否使用本地时间，默认 UTC 时间
LocalTime:  false,
// 是否对日志文件进行压缩归档
Compress:   false,

// 自定义输出位置(看下方tees部分)
Tees: nil,
// Zap Options
ZapOptions: nil,
}

// 输出到指定位置（可选）
// 使用场景：不同级别的日志写入到不同的文件中
tees := []flog.TeeOption{
// 输出到控制台
{
// 如果使用此选项且 flog.Options.Console = true，那么控制台将会输出两条一样的日志信息
Out: os.Stdout,
},
// 输出到文件（可以使用lumberjack库来实现日志轮转）
// 示例：当日志级别是 Error 时将日志写入到 logs/error.log 文件
{
Out: &lumberjack.Logger{
Filename: "logs/error.log",
},
// 返回true才会启用
LevelEnablerFunc: func (level flog.Level) bool {
return level == flog.ErrorLevel
},
},
}

opt.Tees = tees

// 初始化
flog.Init(opt)
// 刷新缓存
defer flog.Sync()
```

**基本使用**

```go
flog.Debug("message", flog.String("str", "foo"), flog.Int("n", 1))
flog.Info("message", flog.String("str", "foo"), flog.Int("n", 1))
flog.Warn("message", flog.String("str", "foo"), flog.Int("n", 1))
flog.Error("message", flog.String("str", "foo"), flog.Int("n", 1))
flog.Panic("message", flog.String("str", "foo"), flog.Int("n", 1))
flog.Fatal("message", flog.String("str", "foo"), flog.Int("n", 1))
```

**Logger 和 SugaredLogger**

关于 `Logger` 和 `SugaredLogger` 的解释可前往[zap](https://markdown.com.cn) 查看。

简单来说

- Logger：仅支持结构化日志，尽可能避免序列化开销和分配；
- SugaredLogger：跟 `fmt.Sprintf` 用法类似，使用encoding/json和fmt.Fprintf记录大量interface{}日志会使您的应用程序变慢；

如何选择？
> 在性能很好但不是很关键的上下文中，使用 SugaredLogger
> 。它比其他结构化日志记录包快4-10倍，并且`支持结构化和printf风格的日志记录`。

> 在每一次内存分配都很重要的上下文中，使用 Logger 。它甚至比 SugaredLogger
> 更快，内存分配次数也更少，但它只支持`强类型的结构化日志记录`。

**SugaredLogger使用**

```go
sugar := flog.Sugar()
sugar.Infof("name=%s", "A")
// 输出 
// {"level":"info","ts":"2024-12-20 17:28:15","caller":"example/main.go:103","msg":"name=A"}
```

**其他**

> 动态更改日志级别
> ``` go
> flog.SetLevel(flog.ErrorLevel) 
> ```


> 替换默认日志实例
> ``` go
> logger := flog.New()
> flog.ReplaceDefault(logger) 
> ```

> 获取日志实例
> ``` go
> flog.Default()  
> ```

## 服务注册

使用 [etcd](https://etcd.io/) 作为注册中心。

**特点**

- ✅ 使用简单；
- ✅ 支持命名空间(隔离)；
- ✅ 内置断线重连机制；
- ✅ 支持自定义元数据；

**使用**

```go
// 返回一个空闲的端口号
port, err := fit.GetFreePort()
if err != nil {
return
}

// 创建一个grpc服务
rpcServer := grpc.NewServer(opts...)

// 注册服务
pb.RegisterUserServer(rpcServer, &userServer{})

reg, err: = fit.NewRegisterService(fit.RegisterOptions {
// 命名空间，默认使用 default
Namespace: "ht",
// 服务类型，可选 "api" 与 "rpc"，默认rpc
ServiceType: "rpc",
// 注册中心key，通常为服务名(如user)
Key: "user",
// 服务IP，填写 "*" 将自动获取网络出口ip。
IP: "*",
// 服务端口
Port: port,
// 服务离线时最大重试等待时间，不传则一直阻塞等待，直到etcd恢复
//MaxTimeoutRetryTime: time.Second * 9,
// etcd 配置
EtcdConfig: fit.EtcdConfig {
Endpoints: [] string {"127.0.0.1:2379"},
},
// zap 日志配置
Logger: flog.ZapLogger(),
// 租约时间
TimeToLive: 10,
// 自定义元数据
Meta: fit.H {
// 设置服务权重，权重越大，服务被调用的次数越多
"weight": * weight,
}})
if err != nil {
log.Fatal(err)
}

// 注销服务
defer reg.Stop()

quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT)
go func () {
fmt.Println("服务启动成功 ", listen.Addr().String())
if err := rpcServer.Serve(listen); err != nil {
log.Fatalln(err)
}
}()
<-quit
fmt.Println("service close")

```

> 参数说明

| 参数名称                | 类型             | 是否必填 | 参数说明     | 备注                               |
|---------------------|----------------|------|----------|----------------------------------|
| Namespace           | string         |      | 命名空间     | 命名空间相互隔离                         |
| ServiceType         | string         | 否    | 服务类型     | 可选 api 与 rpc，默认rpc               |
| Key                 | string         |      | 注册中心key  | 注册中心key，通常为服务名(如user)            |
| IP                  | string         |      | 服务IP     | 填写 "*" 将自动获取网络出口ip。              |
| Port                | string         |      | 服务端口     |                                  |
| MaxTimeoutRetryTime | time.Duration  |      | 最大重试等待时间 | 服务离线时最大重试等待时间，不传则一直阻塞等待，直到etcd恢复 |
| EtcdConfig          | fit.EtcdConfig |      | etcd 配置  |                                  |
| Logger              | *zap.Logger    |      | zap 日志配置 |                                  |
| TimeToLive          | int            |      | 租约时间     | 默认10s                            |
| Meta                | fit.H          |      | 自定义元数据   |                                  |

## 服务发现

**特点**

- ✅ 支持服务发现；
- ✅ 支持多种方案（直连、etcd等）；
- ✅ 内置多种负载均衡方案（随机、轮询、加权轮询、首个健康的连接）；
- ✅ 友好封装；

### gRPC服务发现

gRPC服务发现主要用到`frpc`包。

**使用**

```go
// etcd
clientV3, err := clientv3.New(clientv3.Config{
Endpoints: []string{"127.0.0.1:2379"},
})
if err != nil {
log.Fatal(err)
}

// 程序初始化时调用此方法进行初始化以完成注册
err = frpc.Init(frpc.RpcClientConf{
// etcd
EtcdClient: clientV3,
// 命名空间(不传默认为default) 
Namespace: "ht",

// TLS身份认证，可使用frpc.DisableTLS()来禁用它。 

// TLS单向认证，只有客户端验证服务器的身份
TLSType: frpc.TLSTypeOneWay,
// 公钥证书文件路径
CertFile: "example/k/server.pem",
// 域名
ServerNameOverride: "www.sourcebuild.cn",

// TLS双向认证，客户端不仅验证服务器的证书，服务器也验证客户端的证书。
TLSType:            frpc.TLSTypeMTLS,
CertFile:           "keys/client.crt",
KeyFile:            "keys/client.key",
CAFile:             "keys/ca.crt",
ServerNameOverride: "sourcebuild.cn",
})

// 参数选项
var opts []frpc.DialOption
// 接收 grpc.DialOption
opts = append(opts, frpc.WithGrpcOption(grpc.WithTransportCredentials(insecure.NewCredentials())))

// 负载均衡器
// 均衡器：选择第一个健康的客户端(gRPC默认负载均衡策略)
opts = append(opts, frpc.WithGrpcOption(grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"pick_first"}`)))
// 均衡器：随机
opts = append(opts, frpc.WithBalancerRandom())
// 均衡器：轮询
opts = append(opts, frpc.WithBalancerRoundRobin())
// 均衡器：加权轮询
opts = append(opts, frpc.WithBalancerWeightRoundRobin())

// 阻塞连接
// 直到建立连接成功。如果不设置，NewClient 会立即返回 Dial，并在后台异步进行连接服务器的过程。
// 这个方法可以确保客户端在发送RPC请求之前已经建立了连接。
//
// 这个方法会一直阻塞，可以调用 frpc.WithCtx() 传入 Context 手动取消连接，
// 也可以调用 frpc.WithTimeoutCtx() 传入一个超时时间，该 Context 仅在设置 frpc.WithBlock() 时有效，用于连接超时处理。
opts = append(opts, frpc.WithBlock())
opts = append(opts, frpc.WithTimeoutCtx())

// 禁用TLS身份验证，如果在frpc.Init使用了TLS(单/双向认证),但是又希望在本次连接中不使用身份验证，可使用该方法来禁用它。
opts = append(opts, frpc.DisableTLS())

// 参数1 target: key(即服务注册时填写的key)
client, err := frpc.NewClient("user", opts...)
if err != nil {
log.Fatalln(err)
}

defer client.Close()

c := pb.NewUserClient(client)
resp, err := c.Login(context.Background(), &pb.LoginRequest{
Username: "zs",
Pwd:      "******",
})
// 工具：判断错误是否是“没有可用的服务”，该错误表示没有找到任何服务。
frpc.IsNotFoundServiceErr(err)
```

**直连**

```go
// 直连方式，直接调用的是 grpc.DialContext
conn, err := frpc.NewDirectClient("127.0.0.1:8888", opts...)
if err != nil {
log.Fatal(err)
}
```

当使用了TLS作为身份验证时，gRPC服务端代码可参考...

## Http 统一规范

### Response

统一http固定响应格式，快速在 handler 层返回响应信息。

> 统一状态码规范：假设我们把http状态码划分为3个，即服务端错误时我们返回500，客户端错误时返回400，请求成功时返回 200。
>
> 除了http状态码外，通常我们还需要一个额外的字段表示业务状态码(code)，当我们认为该请求是客户端错误或服务端错误时，我们可以在该字段上使用不同的业务状态码以区分不同的错误场景。

**统一格式响应体**

成功时(200)的响应结构:

```json5
{
  // 业务状态码，我们用0表示请求通过。
  code: 0,
  // 描述信息
  msg: "操作成功",
  // 返回内容，接收任意类型
  result: {
    "id": 1,
    "sex": 1,
  }
}
```

失败时(400 | 500)的响应结构:

```json5
{
  // 业务状态码，我们用非0表示请求失败。
  code: 10400,
  // 失败描述信息
  err_msg: "账号密码错误",
  // 返回内容，接收任意类型
  result: {
    "id": 1
  }
}
```

**快捷使用**

请求成功(200)

```go
// 使用该方法返回一个表示请求成功的响应体。
fres.OkResp(fres.StatusOK, "查询用户信息成功", fit.H{"id": 100})
// {code:0,msg:"查询用户信息成功",result:{id:100}}
```

服务端错误(500)

```go
// 使用该方法返回一个表示服务端错误的响应体，如果不传第三个参数(err)，默认返回包含‘internal server error’的错误信息
fres.InternalErrResp(10026, "服务异常", errors.New("err"))
// {code:10026,err_msg:"服务异常"}

// 同上用法，唯一区别就是该方法接收一个 Result 字段，最终将数据写入到 result 字段中。
fres.InternalErrRespResult(10026, "服务异常", fit.H{})
// {code:10026,err_msg:"服务异常",result:{}}

// 同上效果，可传入一个状态码，会自动根据该状态码去全局注册的列表中查找状态码对应的描述信息，并最终赋值给err_msg字段。
fres.InternalErrRespStatusCode(10026)
// {code:10026,err_msg:"服务异常"}
```

客户端错误(400)

```go
// 使用该方法返回一个表示客户端错误的响应体，如果不传第三个参数(err)，默认返回包含‘client error’的错误信息
fres.ClientErrResp(10411, "参数错误", errors.New("err"))
// {code:10411,err_msg:"服务异常"}

// 同上用法，唯一区别就是该方法接收一个 Result 字段，最终将数据写入到 result 字段中。
fres.ClientErrRespResult(10411, "参数错误", fit.H{})
// {code:10026,err_msg:"服务异常",result:{}}
```

**在handler层使用**

我们可以在路由层或`handler`层使用固定的代码，这样我们就可以只需要关注业务代码。

```go
func QueryUserInfoHandler(c *gin.Context)  {
// ... 参数处理

// 调用业务逻辑层代码，如果返回的err不为空，表示错误请求。
resp, err := QueryUserInfoLogic()
if err != nil {
// 对应http状态码 400 或 500
fres.ErrJson(c, resp)
} else {
// 对应http状态码 == 200
fres.OkJson(c, resp)
}

// 或者这么写，效果等同于上面的写法
resp, err := QueryUserInfoLogic()
fres.Response(c, resp, err)
}
```

**全局注册code状态码**

```go
// 注册全局状态码
fres.RegisterStatusCode(map[interface{}]string{
10023: "找不到用户信息",
10024: "身份验证失败",
10025: "用户信息过期",
10026: "服务异常",
})

// 根据状态码获取描述信息
fres.StatusCodeDesc(10023) // 找不到用户信息

// 快捷使用
fres.InternalErrRespStatusCode(10026) // {code:10026,err_msg:"服务异常"}
```

## Redis

方便快速的使用redis客户端，使用的是 [go-redis](https://github.com/redis/go-redis) 库。

### 初始化

**单机**

快速连接到redis服务器

```go
fit.NewRedisDefaultClient(addr, username, password, db)
```

自定义配置连接到redis服务器

```go
fit.NewRedisClient(opt)
```

**集群**

快速连接到redis服务器

```go
fit.NewRedisDefaultClusterClient(addrs, username, password)
```

自定义配置连接到redis服务器

```go
fit.NewRedisClusterClient(opt)
```

### 使用客户端

**单机**

使用 `fit.RDB` 访问 `*redis.Client`

```go
// 例如：
fit.RDB.Get(context.Background(), "foo").Result()
```

**集群**

使用 `fit.RCDB` 访问 `*redis.ClusterClient`

```go
// 例如：
fit.RCDB.Get(context.Background(), "foo").Result()
```

## MySQL

方便快速的使用mysql客户端，使用的是 [gorm](https://github.com/go-gorm/gorm) 库。

**值得一提**

- ✅ 结合zap日志输出；
- ✅ 判断查询错误结果是否是RecordNotFoundError；
- ✅ 提供 fit.Model 结构体,相同于gorm.Model,为其增加了json格式；

### 初始化

```go
// 初始化一个日志实例，以便将mysql日志输出至此。
opt := flog.Options{
// 建议使用 Info Warn Error 这三个日志级别。
LogLevel:         flog.InfoLevel,
EncoderConfigType: flog.ProductionEncoderConfig,
// 控制台输出
Console:           false,
// 文件输出，为空表示不输出到文件
Filename: "logs/mysql.log",
}
gormLogger := flog.NewGormLogger(opt)

// gorm.Config 配置
gormConfig := &gorm.Config{
// gorm 自定义日志配置 
// 使用zap作为自定义日志
// 自定义Logger，参考：https://github.com/go-gorm/gorm/blob/master/logger/logger.go
Logger: fit.NewGormZapLogger(gormLogger, fit.GormZapLoggerOption{
// 慢SQL阀值，默认200ms
SlowThreshold: 500 * time.Millisecond,
// 忽略 record not found 错误
IgnoreRecordNotFoundError: true,
// 禁用彩色输出
DisableColorful: false,
}),
}

// 该方法仅传入必要的参数，其他配置使用默认值
err := fit.NewMySQLDefaultClient(fit.MySQLClientOption{
Username: "root",
Password: "12345678",
Protocol: "tcp",
Address:  "127.0.0.1:3306",
DbName:   "user",
// 自定义DSN参数，默认使用 charset=utf8&parseTime=True&loc=Local
Params: nil,
// 不使用连接池，默认启用
DisableConnPool: false,
// 设置空闲连接的最大数量，默认10
MaxIdleConns: 0,
// 设置打开连接的最大数量，默认100
MaxOpenConns: 0,
// 设置可以重复使用连接的最长时间，默认1h
ConnMaxLifetime: 0,
// gorm 配置
Config: gormConfig,
})
if err != nil {
log.Fatal(err)
}

// 该方法接收一个 *gorm.DB 类型，自定义完成初始化后将其传入。
fit.InjectMySQLClient()
```

### 使用

```go
// 使用 fit.DB 访问
fit.DB
```

### fit.Model

```go
fit.Model{}

// {
//   id:0,
//   created_at:time.Time,
//   updated_at:time.Time,
//   deleted_at:time.Time,
// }
```

### RecordNotFoundError 错误

当我们在查询时，如果查询记录为0的话，会返回一个 gorm.ErrRecordNotFound 错误，有时候我们希望忽略该错误(因为它并非是个错误)。

```go
// 如果是 gorm.ErrRecordNotFound(查询记录为0) 错误，则err返回nil
if err := fit.HandleGormQueryError(fit.DB.Take(&user, 10).Error); err != nil {
// ...这里处理其他错误
}
```

```go
// 与 fit.HandleGormQueryError 效果相同，不同的是该方法接收一个 *gorm.DB。
tx, err := fit.HandleGormQueryErrorFromTx(fit.DB.Take(&user, 10))
if err != nil {
// ...这里处理其他错误
return
}

fmt.Println(tx.RowsAffected) // 0 
```

## 字符串操作

**高效拼接字符串**

使用 `bytes.Buffer` 拼接字符串。

```go
fit.StringSplice("A", "=", "B", "=", "C")
// A=B=C

fit.StringSpliceTag("-", "A", "B", "C")
// A-B-C
```

**截取指定长度的字符**

由于中英文长度不一致，一个英文字符和一个中文字符在内存中所占的字节数不同，直接按字节截取会导致中文被截断，例如：

```go
str := "123中国人"
fmt.Println(str[0:4])
// 输出：123�
```

使用

```go
str := "123中国人"
fit.SubStrDecodeRuneInString(str, 1, 4)
// 输出：23中
```

## Web请求参数校验

### 使用

```go
g := gin.New()

// zh 或 en，默认 zh
fit.NewValidator()
g.GET("/foo", func(c *gin.Context) {
var req PageRequest
// 绑定参数到结构体
if err := c.ShouldBind(&req); err != nil {
log.Println(err)
return
}

if err := fit.Validate(req); err != nil {
// ...校验不通过
log.Println(err.Error())
return
}

// ...校验通过
c.JSON()
})

g.Run(":8888")
```

### 初始化

**高效拼接字符串**

使用 `bytes.Buffer` 拼接字符串。

```go
fit.StringSplice("A", "=", "B", "=", "C")
// A=B=C

fit.StringSpliceTag("-", "A", "B", "C")
// A-B-C
```

**截取指定长度的字符**

由于中英文长度不一致，一个英文字符和一个中文字符在内存中所占的字节数不同，直接按字节截取会导致中文被截断，例如：

```go
str := "123中国人"
fmt.Println(str[0:4])
// 输出：123�
```

使用

```go
str := "123中国人"
fit.SubStrDecodeRuneInString(str, 1, 4)
// 输出：23中
```

## 防止缓存击穿

> 引用库: golang.org/x/sync/singleflight

**示例代码**

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

**同步阻塞**

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

**异步返回**

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

**异步返回/超时控制**

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

## RabbitMQ

### 使用

```go
// 全局设置
fit.GlobalSetRabbitMQUrl("amqp://guest:guest@127.0.0.1:5672")
//mq, err := fit.NewRabbitMQ()
//defer mq.Close()

//单独设置rabbitMQ地址
mq, err := fit.NewRabbitMQ("amqp://guest:guest@127.0.0.1:5672")
if err != nil {
log.Fatal(err)
}
//释放资源
defer mq.Close()
```

### 获取空闲端口

```go
port, err := fit.GetFreePort()
if err != nil {
return
}
```

### 获取出口IP地址

```go
ip, err := fit.GetOutBoundIP()
if err != nil {
return
}
```

## 网络

### 获取空闲端口

```go
port, err := fit.GetFreePort()
if err != nil {
return
}
```

### 获取出口IP地址

```go
ip, err := fit.GetOutBoundIP()
if err != nil {
return
}
```

## 随机

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

## 加密

### 密码加密

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

### MD5加密

```go
fit.MD5encryption("123456")
```

## 配置文件

使用 [viper](https://github.com/spf13/viper) 库

### 基础使用

```go
func init() {
flag.Int("service.port", 5002, "service port cannot be empty")
}

func main() {
// 加载配置文件，支持yaml、json、ini等文件
// isUseParam: 是否开启命令行参数,默认false
err := fit.NewReadInConfig("config.yaml", true)
if err != nil {
return
}

// 使用
fmt.Println(viper.Get("service.port")) //5002
}
```

### 动态配置

...

## 时间

时间操作推荐使用 [carbon](https://github.com/dromara/carbon) 库。

```go
// 获取此刻到明日凌晨00：00的时间差
t := fit.BeforeDawnTimeDifference()

// 当前是否超过了给定时间
t := fit.SpecifiedTimeExceeded()

// 完整时间
t := fit.GetFullTime(time.Now().Unix())
fmt.Println(t) //2022-06-14 21:51:04

t := fit.GetHMS(time.Now().Unix())
fmt.Println(t) //21:51:55

t := fit.GetMS(time.Now().Unix())
fmt.Println(t) //21:52
```

## 金额/数字

金额/小数操作推荐使用 [decimal](https://github.com/shopspring/decimal) 库。

## JWT

使用 [golang-jwt v5](github.com/golang-jwt/jwt) 库。