package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/source-build/go-fit"
	"github.com/source-build/go-fit/flog"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// 模拟网络故障的测试程序
func main() {
	fmt.Println("=== 网络故障模拟测试工具 ===")
	fmt.Println()
	// 使用代理端口
	fmt.Println("1. 启动代理: go run main.go proxy")
	fmt.Println("2. 启动测试: go run main.go test")
	fmt.Println("3. 在代理窗口按 'd' 断开连接，按 'c' 恢复连接")
	fmt.Println()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "proxy":
			startProxy()
		case "test":
			startTest()
		default:
			fmt.Println("用法: go run network_simulation.go [proxy|test]")
		}
	} else {
		fmt.Println("请选择运行模式:")
		fmt.Println("go run main.go proxy  # 启动代理")
		fmt.Println("go run main.go test   # 启动测试")
	}
}

// 启动TCP代理，可以手动控制连接
func startProxy() {
	fmt.Println("🚀 启动TCP代理 (localhost:12379 -> localhost:2379)")
	fmt.Println("按 'd' 断开连接，按 'c' 恢复连接，按 'q' 退出")

	listener, err := net.Listen("tcp", ":12379")
	if err != nil {
		log.Fatal("代理启动失败:", err)
	}
	defer listener.Close()

	connected := true

	// 监听键盘输入
	go func() {
		var input string
		for {
			fmt.Scanln(&input)
			switch input {
			case "d":
				connected = false
				fmt.Println("🔴 连接已断开")
			case "c":
				connected = true
				fmt.Println("🟢 连接已恢复")
			case "q":
				fmt.Println("退出代理")
				os.Exit(0)
			}
		}
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(clientConn net.Conn) {
			defer clientConn.Close()

			for {
				if !connected {
					time.Sleep(time.Second)
					continue
				}

				// 连接到真实的etcd
				serverConn, err := net.Dial("tcp", "localhost:2379")
				if err != nil {
					return
				}

				// 双向转发数据
				go func() {
					defer serverConn.Close()
					buf := make([]byte, 4096)
					for {
						if !connected {
							return
						}
						n, err := clientConn.Read(buf)
						if err != nil {
							return
						}
						_, err = serverConn.Write(buf[:n])
						if err != nil {
							return
						}
					}
				}()

				buf := make([]byte, 4096)
				for {
					if !connected {
						serverConn.Close()
						break
					}
					n, err := serverConn.Read(buf)
					if err != nil {
						serverConn.Close()
						break
					}
					_, err = clientConn.Write(buf[:n])
					if err != nil {
						serverConn.Close()
						break
					}
				}
			}
		}(conn)
	}
}

// 启动服务注册测试程序
func startTest() {
	// 初始化日志
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

	fmt.Println("🧪 启动服务注册测试")
	fmt.Println("连接到代理端口: localhost:12379")

	// 创建服务注册，连接到代理端口
	reg, err := fit.NewRegisterService(fit.RegisterOptions{
		Namespace:   "ht",
		ServiceType: "rpc",
		Key:         "test-service",
		IP:          "127.0.0.1",
		Port:        "8080",
		EtcdConfig: clientv3.Config{
			Endpoints:   []string{"127.0.0.1:12379"}, // 连接到代理端口
			DialTimeout: time.Second * 5,
		},
		// 续约时间，短租约便于测试
		TimeToLive: 10,
		// 服务断线最大超时重试次数，0表示无限次数(推荐)
		MaxRetryAttempts: 0,
		Logger:           flog.ZapLogger(),
		Meta: fit.H{
			"test": "network-simulation",
		},
	})
	if err != nil {
		log.Fatal("服务注册失败:", err)
	}

	fmt.Println("✅ 服务注册成功")

	// 监听退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 返回一个chan，当etcd离线后重连机制结束时触发
	go func() {
		<-reg.ListenQuit()
		// TODO 应该在此处理停止应用程序的逻辑
		quit <- syscall.SIGINT
		fmt.Println("❌ 服务因网络问题退出")
	}()

	fmt.Println("现在可以在代理窗口按 'd' 断开连接，按 'c' 恢复连接来测试重连机制")
	<-quit
	fmt.Println("停止服务...")
	reg.Stop()
}
