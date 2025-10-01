package main

import (
	"fmt"
	"log"
	"time"

	"github.com/source-build/go-fit/fapi"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	// 创建fapi客户端，支持多服务发现
	client, err := fapi.NewClient(fapi.Options{
		EtcdConfig: clientv3.Config{
			Endpoints: []string{"127.0.0.1:2379"},
		},
		Namespace:           "default",
		DefaultBalancerType: fapi.RoundRobin, // 默认负载均衡器
		// 单独为各服务设置均衡器
		ServiceBalancers: map[string]fapi.BalancerType{
			"user":    fapi.WeightedRoundRobin, // user服务使用加权轮询
			"order":   fapi.LeastConnections,   // order服务使用最少连接
			"payment": fapi.ConsistentHash,     // payment服务使用一致性哈希
		},
	})
	if err != nil {
		log.Fatal("创建fapi客户端失败:", err)
	}
	defer client.Close()

	// 演示多服务发现功能
	demonstrateMultiServiceDiscovery(client)

	// 演示不同服务的负载均衡
	demonstrateServiceSpecificBalancing(client)
	//
	//
	//// 演示高级功能
	//demonstrateAdvancedFeatures(client)
}

// 演示多服务发现功能
func demonstrateMultiServiceDiscovery(client *fapi.Client) {
	fmt.Println("\n=== 多服务发现演示 ===")

	// 获取所有已发现的服务
	serviceNames := client.GetAllServiceNames()
	fmt.Printf("发现的服务: %v\n", serviceNames)

	// 获取每个服务的详细信息
	for _, serviceName := range serviceNames {
		count := client.GetServiceCount(serviceName)
		balancerName, _ := client.GetServiceLoadBalancerName(serviceName)
		fmt.Printf("服务 %s: %d 个实例, 负载均衡器: %s\n", serviceName, count, balancerName)

		// 获取服务的所有实例
		if services, err := client.GetAllServices(serviceName); err == nil {
			for i, service := range services {
				fmt.Printf("  实例 %d: %s (权重: %d)\n", i+1, service.GetAddress(), service.GetWeight())
			}
		}
	}
}

// 演示不同服务的负载均衡
func demonstrateServiceSpecificBalancing(client *fapi.Client) {
	fmt.Println("\n=== 服务特定负载均衡演示 ===")

	services := []string{"user", "order", "payment", "system"}

	for _, serviceName := range services {
		if !client.HasService(serviceName) {
			fmt.Printf("服务 %s 不可用，跳过\n", serviceName)
			continue
		}

		fmt.Printf("\n--- %s 服务负载均衡测试 ---\n", serviceName)
		balancerName, _ := client.GetServiceLoadBalancerName(serviceName)
		fmt.Printf("负载均衡器: %s\n", balancerName)

		// 连续选择5次服务
		for i := 0; i < 5; i++ {
			service, err := client.SelectService(serviceName)
			if err != nil {
				fmt.Printf("选择服务失败: %v\n", err)
				continue
			}

			fmt.Printf("第%d次选择 -> %s\n", i+1, service.GetAddress())

			// 如果是最少连接负载均衡器，模拟释放连接
			if balancerName == "least_connections" {
				go func(svcName, svcKey string) {
					time.Sleep(200 * time.Millisecond)
					client.ReleaseConnection(svcName, svcKey)
					fmt.Printf("  释放连接: %s\n", svcKey)
				}(serviceName, service.GetKey())
			}

			time.Sleep(100 * time.Millisecond)
		}
	}
}

// 演示高级功能
func demonstrateAdvancedFeatures(client *fapi.Client) {
	fmt.Println("\n=== 高级功能演示 ===")

	// 1. 一致性哈希测试
	if client.HasService("payment") {
		fmt.Println("\n一致性哈希测试 (payment服务):")
		testKey := "user_12345"

		for i := 0; i < 5; i++ {
			service, err := client.SelectServiceWithKey("payment", testKey)
			if err != nil {
				fmt.Printf("选择服务失败: %v\n", err)
				continue
			}
			fmt.Printf("  Key '%s' -> %s\n", testKey, service.GetAddress())
		}
	}

	// 2. IP哈希测试
	fmt.Println("\nIP哈希测试:")
	testIP := "192.168.1.100"

	for _, serviceName := range client.GetAllServiceNames() {
		// 临时切换到IP哈希
		originalBalancer, _ := client.GetServiceGroup(serviceName)
		if originalBalancer != nil {
			client.SetServiceLoadBalancer(serviceName, fapi.NewIPHashBalancer())

			service, err := client.SelectServiceWithIP(serviceName, testIP)
			if err != nil {
				fmt.Printf("服务 %s IP哈希选择失败: %v\n", serviceName, err)
			} else {
				fmt.Printf("  服务 %s, IP '%s' -> %s\n", serviceName, testIP, service.GetAddress())
			}

			// 恢复原来的负载均衡器
			//client.SetServiceLoadBalancer(serviceName, originalBalancer.loadBalancer)
		}
	}

	// 3. 健康服务选择测试
	fmt.Println("\n健康服务选择测试:")
	for _, serviceName := range client.GetAllServiceNames() {
		service, err := client.SelectHealthyService(serviceName)
		if err != nil {
			fmt.Printf("服务 %s 无健康实例: %v\n", serviceName, err)
		} else {
			fmt.Printf("  服务 %s 健康实例: %s\n", serviceName, service.GetAddress())
		}
	}

	// 4. 等待服务可用测试
	fmt.Println("\n等待服务可用测试:")
	err := client.WaitForService("nonexistent", 1*time.Second)
	if err != nil {
		fmt.Printf("等待不存在的服务超时: %v\n", err)
	}

	if len(client.GetAllServiceNames()) > 0 {
		serviceName := client.GetAllServiceNames()[0]
		err = client.WaitForService(serviceName, 1*time.Second)
		if err != nil {
			fmt.Printf("等待服务 %s 失败: %v\n", serviceName, err)
		} else {
			fmt.Printf("服务 %s 可用\n", serviceName)
		}
	}
}

// 演示服务组详细信息
func demonstrateServiceGroupDetails(client *fapi.Client) {
	fmt.Println("\n=== 服务组详细信息 ===")

	for _, serviceName := range client.GetAllServiceNames() {
		serviceGroup, err := client.GetServiceGroup(serviceName)
		if err != nil {
			fmt.Printf("获取服务组 %s 失败: %v\n", serviceName, err)
			continue
		}

		fmt.Printf("\n服务组: %s\n", serviceGroup.String())

		// 获取健康服务数量
		healthyServices := serviceGroup.GetHealthyServices()
		fmt.Printf("  健康实例数: %d/%d\n", len(healthyServices), serviceGroup.GetServiceCount())

		// 显示最后使用时间
		fmt.Printf("  最后使用时间: %s\n", serviceGroup.GetLastUsed().Format("2006-01-02 15:04:05"))
	}
}
