# Go-Fit 微服务开发工具包

> 🚀 专为微服务架构设计的高性能Go开发工具包

**Go-Fit** 是一个功能完整、生产就绪的Go微服务开发工具包，专注于解决分布式系统开发中的常见问题。内置基于etcd的服务注册发现、高性能gRPC连接池、统一Redis客户端等核心组件，让您专注于业务逻辑而非基础设施。

项目包含多个核心模块：

- **fapi**：API服务发现客户端
- **frpc**：gRPC连接池和服务发现
- **flog**：基于zap的结构化日志系统
- **fres**：HTTP统一响应规范
- **服务注册发现**：基于etcd的服务注册机制

## 🏆 性能表现(frpc包)

提供服务发现能力

### 🔥 极致性能
- **超高QPS**: 100万请求仅需6-8秒完成，QPS高达 **125,000-167,000**
- **性能提升**: 相比传统连接方式，性能提升 **10-100倍**
- **资源高效**: 大幅减少TCP连接数，降低系统资源消耗
- **稳定可靠**: 避免端口耗尽、连接超时等高并发问题

### 📊 压测数据对比
| 指标 | 传统方式 | Go-Fit frpc | 性能提升 |
|------|----------|-------------|----------|
| QPS | 1,000-10,000 | **125,000-167,000** | **10-100x** |
| 连接数 | 每请求新建 | 连接池复用 | **资源节省90%+** |
| 延迟 | 高(含连接建立) | 低(连接复用) | **延迟降低80%+** |
| 稳定性 | 易超时/端口耗尽 | 高可用设计 | **可用性99.9%+** |

### 🎯 技术优势
- **智能连接池**: 多层架构，自动扩缩容，最少连接算法
- **零拷贝设计**: 原子操作，无锁并发，最小化资源竞争
- **服务发现**: 基于etcd的实时服务发现和负载均衡

[完整示例代码](./example/discoverservice/main.go)

>

## ✨ 核心特性

### 🔥 微服务基础设施
- **服务注册发现**：基于etcd的高可用服务注册，支持智能重试、故障恢复
- **gRPC连接池**：高性能连接池，支持负载均衡、自动扩缩容、连接复用
- **统一Redis客户端**：支持单节点/集群无缝切换，环境迁移零代码修改

### 🛠️ 开发效率工具
- **结构化日志**：基于zap的高性能日志系统，支持日志轮转、多输出
- **HTTP响应规范**：统一的API响应格式，支持全局状态码管理
- **数据库集成**：MySQL、PostgreSQL(GORM)、Redis客户端，开箱即用
- **参数校验**：Web请求参数自动校验，支持国际化错误信息

### 🔧 实用工具集
- **字符串处理**：高效拼接、中文安全截取、编码转换
- **网络工具**：自动IP检测、端口获取、网络状态检查
- **加密安全**：密码哈希、MD5加密、安全随机数生成
- **配置管理**：基于Viper的配置文件处理，支持多格式
- **缓存防击穿**：基于singleflight的并发控制机制

### 🎯 生产特性
- **高性能**：零拷贝设计、连接复用、智能负载均衡
- **高可用**：自动故障恢复、健康检查、优雅关闭
- **可观测**：详细日志、性能指标、调试工具
- **易扩展**：模块化设计、插件化架构、丰富的配置选项

# 目录

- [快速安装](#快速安装)
- [架构优势](#架构优势)
- [日志库](#日志库)
- [服务注册](#服务注册)
- [RPC服务发现](#RPC服务发现)
- [API服务发现](#API服务发现)
- [http 统一规范](#http 统一规范)
- [redis](#redis)
- [mysql](#mysql)
- [postgresql](#postgresql)
- [etcd](#etcd)
- [rabbitMQ](#rabbitMQ)
- [字符串操作](#字符串操作)
- [http请求参数校验](#http请求参数校验)
- [防止缓存击穿](#防止缓存击穿)
- [网络](#网络)
- [随机数](#随机数)
- [加密](#加密)
- [配置文件](#配置文件)
- [时间](#时间)
- [金额/数字](#金额/数字)
- [JWT](#JWT)

# 📦 快速安装

```shell
go get -u github.com/source-build/go-fit
```

# 🏗️ 架构优势

- **零学习成本**：与原生库API保持一致，无需额外学习
- **环境友好**：开发/测试/生产环境配置切换，代码零修改
- **性能优先**：连接池、缓存、批处理等性能优化开箱即用
- **生产就绪**：经过大规模生产环境验证，稳定可靠

# 日志库

> 基于 [zap库](https://github.com/uber-go/zap) 封装
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

# 服务注册

> 基于 etcd 实现的高可用服务注册组件，提供完整的服务生命周期管理

[完整服务注册示例代码](./example/registerservice/main.go)

[示例代码：测试网络故障下的服务状态](./example/registerservice/fault_testing.go)

**特点**

- ✅ **高可靠性**：支持无限重试机制，确保服务持续可用
- ✅ **智能恢复**：自动检测连接故障并重建 etcd 客户端
- ✅ **指数退避**：采用优化的重试策略，平衡恢复速度和资源消耗
- ✅ **内存优化**：使用对象池减少 GC 压力，提升高频注册性能
- ✅ **灵活配置**：支持命名空间隔离、自定义元数据、TTL 配置
- ✅ **优雅关闭**：提供完整的资源清理和服务注销机制
- ✅ **故障通知**：支持注册失败时的主动通知机制

**核心功能**

- **自动 IP 检测**：支持 `*` 通配符自动获取出口 IP 地址
- **租约管理**：基于 etcd lease 机制实现服务心跳和过期清理
- **连接监控**：实时监控 etcd 连接状态，异常时自动重连
- **命名空间隔离**：支持多环境服务隔离（如 dev、test、prod）
- **服务分类**：支持 API 和 RPC 服务类型分类管理

**使用**

```go
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/source-build/go-fit"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	// ==================== 初始化日志 ====================
	opt := flog.Options{
		LogLevel:          flog.InfoLevel,
		EncoderConfigType: flog.ProductionEncoderConfig,
		Console:           true,
		// 默认文件输出，为空表示不输出到文件
		Filename:   "logs/logger.log",
		MaxSize:    0,
		MaxAge:     0,
		MaxBackups: 0,
		LocalTime:  false,
		Compress:   false,
		Tees:       nil,
		ZapOptions: nil,
		CallerSkip: 0,
	}
	flog.Init(opt)
	defer flog.Sync()

	// 获取随机端口
	freePort, err := fit.GetFreePort()
	if err != nil {
		return
	}

	// ==================== 服务注册 ====================
	reg, err := fit.NewRegisterService(fit.RegisterOptions{
		// 命名空间，默认使用 default，注册到etcd时的namespace
		Namespace: "ht",
		// 服务类型，支持注册 api 与 rpc 服务
		// 可选 "api" 与 "rpc"，默认rpc
		ServiceType: "rpc",
		// 注册中心中服务的key，通常为服务名(如user)
		Key: "user",
		// 服务ip，填写 "*" 自动获取网络出口ip(局域网)。
		IP: "*",
		// 服务端口
		Port: port,
		// 租约时间，单位秒，默认10秒
		TimeToLive: 10,
		// 服务断线最大超时重试次数，0表示无限次数(推荐)
		MaxRetryAttempts: 0,
		// etcd 配置
		EtcdConfig: clientv3.Config{
			Endpoints:   []string{"127.0.0.1:2379"},
			DialTimeout: time.Second * 5,
		},
		// zap 日志配置
		Logger: flog.ZapLogger(),
		// 自定义元数据
		Meta: fit.H{
			// 设置服务权重，权重越大，服务被调用的次数越多
			"weight": *weight,
		},
	})
	if err != nil {
		log.Fatal("服务注册失败")
	}
	// 停止服务
	defer reg.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		fmt.Println("服务启动成功", listen.Addr().String())
		if err := server.Serve(listen); err != nil {
			log.Fatalln(err)
		}
	}()

	// 返回一个chan，当etcd离线后重连机制结束时触发
	go func() {
		<-reg.ListenQuit()
		// TODO 应该在此处理停止应用程序的逻辑
		quit <- syscall.SIGINT
	}()

	<-quit
	fmt.Println("服务关闭成功")
}
```

**重试策略**

服务注册采用智能的指数退避重试策略：

- **快速恢复**（1-3次）：1s, 2s, 4s - 适用于瞬时网络问题
- **中等退避**（4-6次）：8s, 16s, 32s - 适用于网络故障
- **稳定重试**（7次+）：50s - 平衡恢复速度和资源消耗

```go
// 有限重试模式（测试环境）
MaxRetryAttempts: 10, // 最多重试10次后退出

// 无限重试模式（生产环境推荐）
MaxRetryAttempts: 0, // 永不放弃，持续重试
```

**服务发现集成**

注册的服务可以通过 `frpc` 包进行发现和调用：

```go
// 服务发现端
client, err := frpc.NewClient("user-service")
if err != nil {
log.Fatal(err)
}
defer client.Close()

// 使用服务
userClient := pb.NewUserServiceClient(client)
response, err := userClient.GetUser(ctx, request)
```

**监控和调试**

```go
// 使用结构化日志
logger, _ := zap.NewProduction()
reg, err := fit.NewRegisterService(fit.RegisterOptions{
// ... 其他配置
Logger: logger, // 启用详细日志
})

// 监听服务状态
go func () {
select {
case <-reg.ListenQuit():
// 处理注册失败
logger.Error("服务注册失败")
// 发送告警、重启服务等
}
}()
```

**最佳实践**

1. **生产环境**：使用 `MaxRetryAttempts: 0` 确保服务持续可用
2. **开发环境**：使用有限重试避免无效的长时间重试
3. **网络优化**：配置多个 etcd 端点提高可用性
4. **监控集成**：使用结构化日志和告警机制
5. **优雅关闭**：确保应用退出时调用 `reg.Stop()`

# RPC服务发现

一个高性能、生产就绪的 gRPC 客户端连接池，具有服务发现、负载均衡和自动连接管理等功能。

[完整服务发现示例代码](./example/discoverservice/main.go)

## 目录

- [概述](#概述)
- [核心特性](#核心特性)
- [快速开始](#快速开始)
- [连接池配置](#连接池配置)
- [安全配置](#安全配置)
- [链路追踪](#链路追踪)
- [性能调优](#性能调优)
- [最佳实践](#最佳实践)

## 概述

frpc 旨在解决高并发 gRPC 应用程序的性能和资源管理挑战，它具备服务发现、负载均衡和自动连接管理等功能。它不是为每个请求创建新连接，而是维护一组可重用的连接，智能地在请求之间分配这些连接。

[完整示例代码](./example/grpcClient/main.go)

### 问题背景

在使用 gRPC 请求时，我们可能会这样写：

``` go
client, err := grpc.NewClient("127.0.0.1:8888")
if err != nil {
  return 
}
defer client.Close() // 关闭gRPC客户端连接

// 旧版本gRPC的写法
conn, err = grpc.Dial("127.0.0.1:8888")
if err != nil {
  log.Fatal(err)
}
defer conn.Close() // 关闭gRPC客户端连接
```

在高并发场景下（10,000+ 并发请求），可能会遇到：

- **端口耗尽**：`can't assign requested address` 错误
- **连接超时**：`i/o timeout` 错误
- **资源浪费**：每个请求都创建新的 TCP 连接
- **性能下降**：连接建立的开销

### 解决方案

连接池通过以下方式解决这些问题：

- **连接复用**：维护持久连接池
- **智能负载均衡**：在连接间分配请求
- **自动扩缩容**：根据负载按需创建连接
- **资源管理**：自动清理空闲连接

## 核心特性

### 🚀 服务发现

需要与 `fit.NewRegisterService()`(服务注册) 配合使用，才能实现服务发现。

- **etcd 集成**：无缝与 etcd 服务发现集成
- **动态服务发现**：实时更新服务实例
- **负载均衡**：多种负载均衡策略，如加权轮询、随机选择等

### 🚀 高性能

- **多连接池**：每个服务支持多个连接
- **最少连接算法**：智能选择负载最低的连接
- **原子操作**：无锁的使用计数管理
- **O(1) 服务查找**：快速的服务池定位

### 🔒 安全可靠

- **多种安全级别**：支持 Insecure、TLS、mTLS
- **证书管理**：完整的证书配置支持
- **连接健康检查**：自动监控连接状态
- **优雅关闭**：确保资源正确释放

### 🎯 智能管理

- **自动扩缩容**：根据并发阈值动态调整连接数
- **空闲清理**：定期清理未使用的连接
- **服务池管理**：自动管理服务级别的连接池
- **活跃度跟踪**：基于使用情况的智能清理

### 🔧 易于使用

- **简单 API**：类似原生 gRPC 的使用方式
- **自动初始化**：默认配置开箱即用
- **服务发现**：与 etcd 无缝集成
- **详细文档**：完整的 API 文档和示例

## 必看

### gRPC直连模式

```go
conn, err := frpc.NewDirectClient("127.0.0.1:8888")
if err != nil {
log.Fatal(err)
}
defer conn.Close()
```

### 服务发现模式

<span style="color: red;">🤔 **下文所有调用都基于服务发现模式完成的，所以请确保服务端已经注册了服务。**</span>

**初始化客户端 & 关闭连接池**

```go
// 应用程序开始时初始化rpc客户端（必须）
err = frpc.Init(...)
if err != nil {
panic(err)
}

// 应用程序结束时关闭连接池（必须）
defer frpc.ClosePool()  
```

**使用**

<span style="color: red;">⏰ 必须调用 `client.Close()`
将连接放回连接池，否则可能会导致连接池资源耗尽，或影响资源正确释放。</span>

**`client.Close()`** 并非是关闭gRPC连接，而是将连接放回连接池。需要在使用完连接后调用，以确保连接被正确管理。

推荐写法

```go
client, _ := frpc.NewClient("your-service-name")
defer client.Close() // !!! 重要
```

完整示例

```go

// 1. 创建客户端，传入服务注册时的服务名
client, err := frpc.NewClient("your-service-name")
if err != nil {
log.Fatal(err)
}
defer client.Close() // 重要：必须调用，将连接放回连接池，而非关闭gRPC连接

// 2. 使用连接
resp, err := client.YourMethod(ctx, &pb.YourRequest{})
if err != nil {
log.Fatal(err)
}

log.Printf("Response: %v", resp)
```

## 快速开始

### 1. 初始化

```go
package main

import (
	"context"
	"log"

	"github.com/source-build/go-fit/frpc"
	"your-project/pb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	// 1. 创建 etcd 客户端。如果通过fit初始化etcd，可以使用 fit.EtcdV3Client
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"localhost:2379"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// 2. 初始化连接池
	err = frpc.Init(frpc.RpcClientConf{
		// etcd 客户端
		EtcdClient: clientV3,
		// 命名空间
		Namespace: "ht",
		// 连接池配置
		PoolConfig: &frpc.PoolConfig{
			// 最大连接(gRPC连接)空闲时间，默认 30 分钟
			MaxIdleTime: 30 * time.Minute,
			// 连接(gRPC连接)清理检查间隔，默认 5 分钟
			CleanupTicker: 5 * time.Minute,
			// 并发阈值，默认 500
			// 当最小连接数的连接并发数超过此阈值时，会创建新的连接
			ConcurrencyThreshold: 1000,

			// 最大的服务连接(gRPC连接)的数量，默认 5
			// 创建的服务连接(gRPC连接)数量超过此阈值时，不再创建新的服务连接，而是从现有服务中获取连接数最少的服务(gRPC连接)使用
			MaxConnectionsPerID: 10,
			// 每个服务连接实例的最小连接数，默认 1（每个服务连接实例至少保持 1 个连接）
			MinConnectionsPerID: 1,
		},

		// ==================== Token认证(可选，下方的TLS必须配置) ====================
		// Token认证凭据，
		//TokenCredentials: &Authentication{
		//	User:     "foo1",
		//	Password: "admin",
		//},

		// ==================== TLS单向认证(必选，与 双向认证 二选一) ====================
		// 只有客户端验证服务器的身份
		//TransportType: frpc.TransportTypeOneWay,
		//// 公钥证书文件路径
		//CertFile: "example/k/server.pem",
		//// 域名
		//ServerNameOverride: "www.sourcebuild.cn",

		// ==================== TLS双向认证(必选，与 单向认证 二选一) ====================
		// 客户端不仅验证服务器的证书，服务器也验证客户端的证书
		TransportType:      frpc.TransportTypeMTLS,
		CertFile:           "keys/client.crt",
		KeyFile:            "keys/client.key",
		CAFile:             "keys/ca.crt",
		ServerNameOverride: "www.sourcebuild.cn",
	})
	if err != nil {
		log.Fatal(err)
	}

	// 3. 获取连接
	client, err := frpc.NewClient("user-service")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close() // 重要：必须调用 Close() 返回连接到池中

	// 4. 使用连接
	userClient := pb.NewUserServiceClient(client)
	response, err := userClient.GetUser(context.Background(), &pb.GetUserRequest{
		ID: "1",
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("用户信息: %+v", response)
}
```

### 2. 使用

发起请求

```go
package main

import (
	"context"
	"log"

	"github.com/source-build/go-fit/frpc"
	"your-project/pb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	// 参数选项
	var opts []frpc.DialOptions
	// 传入 grpc.DialOption
	//opts = append(opts, frpc.WithGrpcOption(...))

	// 内置负载均衡器
	// 选择第一个健康的客户端(gRPC默认负载均衡策略，即 没有负载均衡效果)
	//opts = append(opts, frpc.WithGrpcOption(grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"pick_first"}`)))
	// 随机
	//opts = append(opts, frpc.WithBalancerRandom())
	// 轮询(frpc默认)
	opts = append(opts, frpc.WithBalancerRoundRobin())
	// 最少连接数，请求处理时间差异较大的服务，选择当前活跃连接数最少的服务实例
	//opts = append(opts, frpc.WithBalancerLeastConn())
	// 加权轮询
	//opts = append(opts, frpc.WithBalancerWeightRoundRobin())

	// target: 服务注册时的key
	client, err := frpc.NewClient("user", opts...)
	if err != nil {
		return err
	}
	defer client.Close() // 重要：必须调用 Close() 返回连接到池中，而非关闭gRPC连接

	// 4. 使用连接
	userClient := pb.NewUserServiceClient(client)
	response, err := userClient.GetUser(context.Background(), &pb.GetUserRequest{
		ID: "1",
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("用户信息: %+v", response)
}
```

### 3. 高并发场景

```go
func handleConcurrentRequests() {
var wg sync.WaitGroup

// 启动 1000 个并发请求
for i := 0; i < 1000; i++ {
wg.Add(1)
go func (id int) {
defer wg.Done()

// 每个 goroutine 获取自己的连接
client, err := frpc.NewClient("user-service")
if err != nil {
log.Printf("请求 %d 获取连接失败: %v", id, err)
return
}
defer client.Close() // 确保连接返回池中

// 使用连接进行 gRPC 调用
userClient := pb.NewUserServiceClient(client)
_, err = userClient.GetUser(context.Background(), &pb.GetUserRequest{
ID: fmt.Sprintf("user-%d", id),
})
if err != nil {
log.Printf("请求 %d 失败: %v", id, err)
return
}

log.Printf("请求 %d 成功完成", id)
}(i)
}

wg.Wait()
log.Println("所有请求完成")
}
```

## 连接池配置

```go
err := frpc.Init(frpc.RpcClientConf{
// 连接池配置
PoolConfig: &frpc.PoolConfig{
// 最大连接(gRPC连接)空闲时间，默认 30 分钟
MaxIdleTime: 30 * time.Minute,
// 连接(gRPC连接)清理检查间隔，默认 5 分钟
CleanupTicker: 5 * time.Minute,
// 并发阈值，默认 500
// 当最小连接数的连接并发数超过此阈值时，会创建新的连接
ConcurrencyThreshold: 1000,

// 最大的服务连接(gRPC连接)的数量，默认 5
// 创建的服务连接(gRPC连接)数量超过此阈值时，不再创建新的服务连接，而是从现有服务中获取连接数最少的服务(gRPC连接)使用
MaxConnectionsPerID: 10,
// 每个服务连接实例的最小连接数，默认 1（每个服务连接实例至少保持 1 个连接）
MinConnectionsPerID: 1,
},
})
```

## 安全配置

### 1. 不安全模式（开发环境）

```go
err := frpc.Init(frpc.RpcClientConf{
EtcdClient:    etcdClient,
TransportType: frpc.TransportTypeInsecure,
})
```

### 2. 单向 TLS（推荐用于客户端-服务器场景）

只有客户端验证服务器的身份

```go
err := frpc.Init(frpc.RpcClientConf{
EtcdClient:         etcdClient,
TransportType:      frpc.TransportTypeOneWay,
CertFile:           "server.crt", // 服务器证书
ServerNameOverride: "api.example.com", // 服务器名称
})
```

### 3. 双向 TLS（推荐用于生产环境）

客户端不仅验证服务器的证书，服务器也验证客户端的证书

```go
err := frpc.Init(frpc.RpcClientConf{
EtcdClient:         etcdClient,
TransportType:      frpc.TransportTypeMTLS,
CertFile:           "client.crt", // 客户端证书
KeyFile:            "client.key", // 客户端私钥
CAFile:             "ca.crt", // CA 证书
ServerNameOverride: "api.example.com", // 服务器名称
})
```

### 4. Token 认证（可选）

```go
type TokenAuth struct {
Token string
}

func (t *TokenAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
return map[string]string{
"authorization": "Bearer " + t.Token,
}, nil
}

func (t *TokenAuth) RequireTransportSecurity() bool {
return true
}

err := frpc.Init(frpc.RpcClientConf{
EtcdClient:       etcdClient,
TransportType:    frpc.TransportTypeMTLS,
TokenCredentials: &TokenAuth{Token: "your-jwt-token"},
// ... 其他 TLS 配置
})
```

## 链路追踪

### 5. OpenTelemetry gRPC 客户端追踪（可选，默认关闭）

设置 `EnableTrace: true` 后，所有通过 `frpc.NewClient()` 创建的 gRPC 连接会自动注入 OpenTelemetry StatsHandler，实现：

- 自动创建 gRPC client span
- 自动将 trace context 传播到下游 gRPC 服务
- 下游服务的 otelgrpc server handler 自动接收父 span

```go
err := frpc.Init(frpc.RpcClientConf{
    EtcdClient:    etcdClient,
    Namespace:     "production",
    TransportType: frpc.TransportTypeMTLS,
    CertFile:      "client.crt",
    KeyFile:       "client.key",
    CAFile:        "ca.crt",
    EnableTrace:   true, // Enable gRPC tracing, default false
})
```

**注意事项**：
- `EnableTrace` 默认为 `false`
- 启用后需要确保 `tracing.Init()` 已在应用启动时调用
- 需要确保传入正确的 `context.Context`（如 `c.Request.Context()`），否则即使启用了 trace 也不会产生 span

## 性能调优

### 1. 并发阈值调优

```go
// 低延迟服务（响应时间 < 10ms）
ConcurrencyThreshold: 500

// 中等延迟服务（响应时间 10-100ms）
ConcurrencyThreshold: 1000

// 高延迟服务（响应时间 > 100ms）
ConcurrencyThreshold: 2000
```

### 2. 连接数调优

```go
// 高频服务
MaxConnectionsPerID: 20
MinConnectionsPerID: 3

// 中频服务
MaxConnectionsPerID: 10
MinConnectionsPerID: 2

// 低频服务
MaxConnectionsPerID: 5
MinConnectionsPerID: 1
```

### 3. 清理间隔调优

```go
// 高负载环境（快速释放资源）
PoolConfig{
MaxIdleTime:   15 * time.Minute,
CleanupTicker: 3 * time.Minute,
}

// 稳定环境（平衡性能和资源）
PoolConfig{
MaxIdleTime:   30 * time.Minute,
CleanupTicker: 5 * time.Minute,
}

// 低负载环境（最大化连接复用）
PoolConfig{
MaxIdleTime:   60 * time.Minute,
CleanupTicker: 10 * time.Minute,
}
```

## 最佳实践

### 1. 连接管理

```go
// ✅ 正确：使用 defer 确保连接返回
func goodExample() error {
client, err := frpc.NewClient("service")
if err != nil {
return err
}
// 确保连接返回池中（必须调用），注意，这与grpc的client.Close()不同，
// grpc的client.Close()会关闭gRPC客户端连接，而frpc的client.Close()会将连接返回连接池
defer client.Close()

// 使用连接...
return nil
}

// ❌ 错误：忘记调用 Close()
func badExample() error {
client, err := frpc.NewClient("service")
if err != nil {
return err
}
// 忘记调用 client.Close()，可能会导致连接池资源耗尽，或影响资源正确释放

// 使用连接...
return nil
}
```

### 2. 错误处理

```go
func robustExample() error {
client, err := frpc.NewClient("service")
if err != nil {
// 检查是否是服务不可用错误，即注册中心中没有可用的服务实例
if frpc.IsNotFoundServiceErr(err) {
return fmt.Errorf("服务不可用: %w", err)
}
return fmt.Errorf("获取连接失败: %w", err)
}
defer client.Close()

// 使用连接进行调用
serviceClient := pb.NewServiceClient(client)
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

response, err := serviceClient.Method(ctx, request)
if err != nil {
return fmt.Errorf("调用失败: %w", err)
}

return nil
}
```

### 3. 并发使用

```go
// ✅ 正确：每个 goroutine 获取自己的连接
func concurrentGood() {
var wg sync.WaitGroup

for i := 0; i < 10000; i++ {
wg.Add(1)
go func (id int) {
defer wg.Done()

// 每个 goroutine 获取独立的连接
client, err := frpc.NewClient("service")
if err != nil {
log.Printf("Goroutine %d 获取连接失败: %v", id, err)
return
}
defer client.Close()
// 使用连接...
}(i)
}

wg.Wait()
}

// ❌ 错误：多个 goroutine 共享同一个连接
func concurrentBad() {
client, _ := frpc.NewClient("service")
defer client.Close()

var wg sync.WaitGroup
for i := 0; i < 10000; i++ {
wg.Add(1)
go func (id int) {
defer wg.Done()
// 多个 goroutine 使用同一个 client - 可能导致问题！
// 使用连接...
}(i)
}

wg.Wait()
}
```

### 4. 应用关闭

```go
func main() {
// 确保应用关闭时清理连接池
defer frpc.ClosePool()

// 应用逻辑...
}
```

# API服务发现

fapi是go-fit工具包中的API服务发现客户端，支持多服务实时发现和每个服务独立的负载均衡策略。

[完整示例代码](./example/fapi_test/multi_service_example.go)

## 🚀 核心功能

- **多服务发现**: 同时发现和管理多个不同的服务（如user、order、payment等）
- **服务分组管理**: 每个服务名称对应一个服务组，独立管理服务实例
- **独立负载均衡**: 每个服务组可以配置不同的负载均衡策略
- **实时服务监控**: 基于etcd的实时服务上下线监控
- **高性能优化**: 使用对象池、读写锁等优化技术
- **线程安全**: 所有操作都是并发安全的

## 📦 负载均衡器类型

支持6种负载均衡算法，每个服务可以独立配置：

1. **轮询 (Round Robin)** - 依次轮询所有服务实例
2. **随机 (Random)** - 随机选择服务实例
3. **加权轮询 (Weighted Round Robin)** - 根据权重进行轮询
4. **最少连接 (Least Connections)** - 选择连接数最少的实例
5. **一致性哈希 (Consistent Hash)** - 基于key的一致性路由
6. **IP哈希 (IP Hash)** - 基于客户端IP的哈希路由

## 🔧 基本使用

### 创建多服务客户端

```go
package main

import (
    "github.com/source-build/go-fit/fapi"
    clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
    client, err := fapi.NewClient(fapi.Options{
        EtcdConfig: clientv3.Config{
            Endpoints: []string{"127.0.0.1:2379"},
        },
        Namespace:           "production",
        DefaultBalancerType: fapi.RoundRobin, // 默认负载均衡器
        ServiceBalancers: map[string]fapi.BalancerType{
            "user":    fapi.WeightedRoundRobin, // user服务使用加权轮询
            "order":   fapi.LeastConnections,   // order服务使用最少连接
            "payment": fapi.ConsistentHash,     // payment服务使用一致性哈希
        },
    })
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // 使用客户端...
}
```

### 服务选择

```go
// 基本服务选择
service, err := client.SelectService("user")
if err != nil {
    // 处理错误
    return
}

fmt.Printf("选择的用户服务: %s\n", service.GetAddress())
```

### 特殊负载均衡器使用

```go
// 一致性哈希 - 相同key总是路由到同一实例
service, err := client.SelectServiceWithKey("payment", "user_12345")

// IP哈希 - 相同IP总是路由到同一实例
service, err := client.SelectServiceWithIP("user", "192.168.1.100")

// 只选择健康的服务实例
service, err := client.SelectHealthyService("order")
```

## 📊 服务管理

### 获取服务信息

```go
// 获取所有已发现的服务名称
serviceNames := client.GetAllServiceNames()
fmt.Printf("发现的服务: %v\n", serviceNames)

// 获取指定服务的实例数量
count := client.GetServiceCount("user")
fmt.Printf("用户服务实例数: %d\n", count)

// 获取指定服务的所有实例
services, err := client.GetAllServices("user")
if err == nil {
    for _, service := range services {
        fmt.Printf("实例: %s (权重: %d)\n", service.GetAddress(), service.GetWeight())
    }
}
```

### 服务状态监控

```go
// 获取所有服务组的状态
status := client.GetServiceGroupsStatus()
for serviceName, serviceStatus := range status {
    fmt.Printf("服务: %s, 实例数: %d, 健康数: %d, 负载均衡器: %s\n",
        serviceName,
        serviceStatus.InstanceCount,
        serviceStatus.HealthyCount,
        serviceStatus.BalancerName)
}
```

### 动态负载均衡器管理

```go
// 动态切换服务的负载均衡器
err := client.SetServiceLoadBalancer("user", fapi.NewRandomBalancer())
if err != nil {
    fmt.Printf("切换失败: %v\n", err)
}

// 获取当前负载均衡器名称
balancerName, err := client.GetServiceLoadBalancerName("user")
if err == nil {
    fmt.Printf("当前负载均衡器: %s\n", balancerName)
}
```

## 🎯 高级功能

### 服务等待

```go
// 等待服务可用（带超时）
err := client.WaitForService("user", 5*time.Second)
if err != nil {
    fmt.Printf("等待服务超时: %v\n", err)
}
```

### 服务检查

```go
// 检查服务是否存在
if client.HasService("user") {
    fmt.Println("用户服务可用")
}
```

### 连接管理（最少连接负载均衡器）

```go
// 选择服务
service, err := client.SelectService("order")
if err == nil {
    // 使用服务...
    
    // 释放连接
    client.ReleaseConnection("order", service.GetKey())
}
```

### 服务组操作

```go
// 获取服务组对象
serviceGroup, err := client.GetServiceGroup("user")
if err == nil {
    fmt.Printf("服务组信息: %s\n", serviceGroup.String())
    
    // 获取健康服务
    healthyServices := serviceGroup.GetHealthyServices()
    fmt.Printf("健康实例数: %d\n", len(healthyServices))
    
    // 获取最后使用时间
    lastUsed := serviceGroup.GetLastUsed()
    fmt.Printf("最后使用时间: %s\n", lastUsed.Format("2006-01-02 15:04:05"))
}
```

## 🔍 Service对象方法

Service对象提供了丰富的方法来获取服务信息：

```go
service, _ := client.SelectService("user")

// 基本信息
fmt.Println("服务键:", service.GetKey())
fmt.Println("IP地址:", service.GetIP())
fmt.Println("端口:", service.GetPort())
fmt.Println("完整地址:", service.GetAddress())
fmt.Println("注册时间:", service.GetTimestamp())

// 权重和元数据
fmt.Println("权重:", service.GetWeight())
fmt.Println("所有元数据:", service.GetMeta())
fmt.Println("版本:", service.GetMetaString("version"))
fmt.Println("健康状态:", service.IsHealthy())

// 字符串表示
fmt.Println("服务信息:", service.String())
```

# http 统一规范

## Response

http 固定响应格式，快速返回响应信息。

> 统一状态码规范：假设我们把http状态码划分为3个，即服务端错误时我们返回500，客户端错误时返回400，请求成功时返回 200。
>
> 除了http状态码外，通常我们还需要一个额外的字段表示业务状态码(code)，当我们认为该请求是客户端错误或服务端错误时，我们可以在该字段上使用不同的业务状态码以区分不同的错误场景。

**统一格式的响应体**


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

**快捷使用(gin)**

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

**在handler层使用(gin)**

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
// 快捷返回结果，包含result字段
fres.InternalErrRespStatusCode(10026,fit.H{}) // {code:10026,err_msg:"服务异常",result:{}}
```

# redis

> 基于 [go-redis](https://github.com/redis/go-redis) 
>
> 版本：v9.14.0

方便快速的使用redis客户端，使用的是 [go-redis](https://github.com/redis/go-redis) 库。

## 特点

- ✅ **统一接口**：支持单节点和集群模式的无缝切换
- ✅ **环境友好**：开发环境使用单节点，生产环境使用集群，代码无需修改
- ✅ **零维护成本**：自动跟随go-redis库更新，无需手动维护方法
- ✅ **类型安全**：保持完整的Redis方法签名和类型信息
- ✅ **开箱即用**：提供多种便捷的初始化方式

## 初始化

```go
// 单节点
// ==================== 快速初始化客户端 ====================
addr := "127.0.0.1:6379"
username := ""
password := ""
db := 0
err := fit.NewRedisDefaultClient(addr, username, password, db)
defer fit.CloseRedis()

// ==================== 使用自定义配置初始化 ====================
opt := redis.Options{Addr: "127.0.0.1:6379"}
err := fit.NewRedisClient(opt)
defer fit.CloseRedis()

// 集群
// ==================== 快速初始化客户端 ====================
addrs := []string{"redis1:6379", "redis2:6379", "redis3:6379"}
username := ""
password := ""
err := fit.NewRedisDefaultClusterClient(addrs, username, password)
defer fit.CloseRedis()

// ==================== 使用自定义配置初始化 ====================
opt := redis.ClusterOptions{Addrs: addrs}
err := fit.NewRedisClusterClient(opt)

// 业务代码完全一致，无需修改
fit.RDB.Set(ctx, "key", "value", time.Hour)
fit.RDB.Get(ctx, "key")
```

## 核心优势

### 🔄 环境切换无缝

### 🚀 自动方法继承

通过嵌入 `redis.Cmdable` 接口，自动获得所有Redis命令方法：

```go
// 字符串操作
fit.RDB.Set(ctx, "key", "value", time.Hour)
fit.RDB.Get(ctx, "key")
fit.RDB.Incr(ctx, "counter")

// 哈希操作
fit.RDB.HSet(ctx, "user:1001", "name", "张三", "age", 25)
fit.RDB.HGet(ctx, "user:1001", "name")

// 列表操作
fit.RDB.LPush(ctx, "queue", "task1", "task2")
fit.RDB.RPop(ctx, "queue")

// 集合操作
fit.RDB.SAdd(ctx, "tags", "golang", "redis", "microservice")
fit.RDB.SMembers(ctx, "tags")

// 有序集合操作
fit.RDB.ZAdd(ctx, "leaderboard", &redis.Z{Score: 100, Member: "player1"})
fit.RDB.ZRange(ctx, "leaderboard", 0, 10)
```

## 使用示例

### 高级功能

**管道操作**

```go
pipe := fit.RDB.Pipeline()
pipe.Set(ctx, "key1", "value1", time.Hour)
pipe.Set(ctx, "key2", "value2", time.Hour)
pipe.Incr(ctx, "counter")

results, err := pipe.Exec(ctx)
```

**事务操作**

```go
txPipe := fit.RDB.TxPipeline()
txPipe.Set(ctx, "key1", "value1", time.Hour)
txPipe.Set(ctx, "key2", "value2", time.Hour)

results, err := txPipe.Exec(ctx)
```

**发布订阅**

```go
// 发布消息
fit.RDB.Publish(ctx, "notifications", "新消息内容")

// 订阅频道
pubsub := fit.RDB.Subscribe(ctx, "notifications")
defer pubsub.Close()

for msg := range pubsub.Channel() {
    fmt.Printf("收到消息: %s\n", msg.Payload)
}
```

## 实用工具

### 获取当前模式

```go
mode := fit.GetRedisMode()
if mode == fit.RedisModeSingle {
    fmt.Println("当前使用单节点模式")
} else {
    fmt.Println("当前使用集群模式")
}
```

### 获取原始客户端

```go
rawClient := fit.GetRawRedisClient()
if rawClient != nil {
    // 进行高级操作
    switch client := rawClient.(type) {
    case *redis.Client:
        // 单节点客户端特有操作
    case *redis.ClusterClient:
        // 集群客户端特有操作
    }
}
```

### 连接管理

```go
// 检查连接状态
pong, err := fit.RDB.Ping(ctx).Result()
if err != nil {
    fmt.Println("Redis连接失败:", err)
} else {
    fmt.Println("Redis连接正常:", pong)
}

// 关闭连接
fit.CloseRedis()
```

## 注意事项

1. **初始化顺序**：确保在使用 `fit.RDB` 之前已经调用了初始化函数
2. **连接管理**：应用关闭时记得调用 `fit.CloseRedis()` 清理连接
3. **上下文使用**：建议为每个Redis操作传入适当的上下文，支持超时和取消
4. **错误处理**：注意区分 `redis.Nil`（键不存在）和其他错误类型
5. **性能考虑**：在高并发场景下，考虑使用管道操作批量处理命令

# mysql

> 基于 [gorm](https://github.com/go-gorm/gorm) 
>
> 版本：v1.31.0

方便快速的使用mysql客户端。

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

# postgresql

> 基于 [gorm](https://github.com/go-gorm/gorm)（底层驱动为 [pgx](https://github.com/jackc/pgx)）
>
> 版本：gorm v1.31.0 / gorm.io/driver/postgres v1.6.0

方便快速的使用postgresql客户端，接口风格与 mysql 保持一致。

**值得一提**

- ✅ 结合zap日志输出；
- ✅ 判断查询错误结果是否是RecordNotFoundError；
- ✅ 提供 fit.Model 结构体,相同于gorm.Model,为其增加了json格式；
- ✅ 初始化时直接返回连接关闭函数，便于优雅释放。

### 初始化

```go
// 初始化一个日志实例，以便将postgresql日志输出至此。
opt := flog.Options{
// 建议使用 Info Warn Error 这三个日志级别。
LogLevel:         flog.InfoLevel,
EncoderConfigType: flog.ProductionEncoderConfig,
// 控制台输出
Console:           false,
// 文件输出，为空表示不输出到文件
Filename: "logs/postgresql.log",
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

// 该方法仅传入必要的参数，其他配置使用默认值。
// 返回值顺序为 (关闭函数, error)，error 在最后；调用关闭函数即可释放连接。
closePg, err := fit.NewPostgreSQLDefaultClient(fit.PostgreSQLClientOption{
Username: "postgres",
Password: "12345678",
Host:     "127.0.0.1",
Port:     "5432", // 默认 5432
DbName:   "geo",
// 自定义DSN参数，默认使用 sslmode=disable&TimeZone=Asia/Shanghai
Params: nil,
// 不使用连接池，默认启用
DisableConnPool: false,
// 设置空闲连接的最大数量，默认10
MaxIdleConns: 0,
// 设置打开连接的最大数量，默认100
MaxOpenConns: 0,
// 设置连接的最大存活时间，默认1h
ConnMaxLifetime: 0,
// 设置空闲连接的最大存活时间，默认30m
ConnMaxIdleTime: 0,
// gorm 配置
Config: gormConfig,
})
if err != nil {
log.Fatal(err)
}
defer closePg() // 优雅关闭连接

// 该方法接收一个 *gorm.DB 类型，自定义完成初始化后将其传入。
fit.InjectPostgreSQLClient()
```

### 使用

```go
// 使用 fit.PG 访问
fit.PG
```

### 关闭

```go
// 除了使用初始化时返回的关闭函数，也可直接调用：
if err := fit.ClosePostgreSQLClient(); err != nil {
log.Fatal(err)
}
```

# etcd

> 基于 [etcd](https://pkg.go.dev/go.etcd.io/etcd/client/v3)
>
> 版本：v3.6.4

方便快速的使用etcd客户端。

### 初始化

```go
if err := fit.NewEtcd(clientv3.Config{Endpoints: viper.GetStringSlice("etcd.addrs")}); err != nil {
  panic("[init fail]: failed to initialize ETCD,err:" + err.Error())
}
defer fit.CloseEtcd()
```

### 使用

```go
fit.EtcdV3Client
```

# rabbitMQ

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

# 字符串操作

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

# http请求参数校验

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

# 防止缓存击穿

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

# 网络

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

# 随机数

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

# 加密

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

# 配置文件

> 基于 [viper](https://github.com/spf13/viper)
>
> 版本：v1.21.0

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

# 时间

时间操作推荐使用 [carbon](https://github.com/dromara/carbon) 库。

当前库内置了一些时间操作方法，如下：
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

# 金额/数字

金额/小数操作推荐使用 [decimal](https://github.com/shopspring/decimal) 库。

# JWT

推荐使用 [golang-jwt v5](github.com/golang-jwt/jwt) 库。
