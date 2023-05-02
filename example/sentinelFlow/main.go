package main

import (
	"fmt"
	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/source-build/go-fit"
	"log"
	"math/rand"
	"time"
)

var flowRules = []*flow.Rule{
	//Direct + Reject 的流控策略,QPS 10
	{
		Resource:               "some-test",
		Threshold:              10, //流控阈值；如果字段 StatIntervalInMs 是1000(也就是1秒)，那么Threshold就表示QPS，流量控制器也就会依据资源的QPS来做流控
		TokenCalculateStrategy: flow.Direct,
		ControlBehavior:        flow.Reject, //表示流量控制器的控制策略；Reject表示超过阈值直接拒绝，Throttling表示匀速排队
		StatIntervalInMs:       1000,        //规则对应的流量控制器的独立统计结构的统计周期。如果StatIntervalInMs是1000，也就是统计QPS。
	},
	/**
	  内存自适应
	*/
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

func main() {
	err := fit.InitSentinel(fit.SentinelConfig{
		Version: "1.0.1",
		AppName: "cs",
		LogDir:  "./logs",
	})
	if err != nil {
		log.Fatalln(err)
	}
	err = fit.LoadFlowRule(flowRules)
	if err != nil {
		log.Fatalln(err)
	}

	// 模拟内存使用量为1000字节，因此QPS阈值应为1000
	//fmt.Println("内存使用量为999:", new(fitutil.ParseTime).HSM(time.Now().Unix()))
	//system_metric.SetSystemMemoryUsage(999)
	ch := make(chan bool)
	//for i := 0; i < 10; i++ {
	//	go func() {
	//		for {
	//			e, b := sentinel.Entry("some-test1", sentinel.WithTrafficType(base.Inbound))
	//			if b != nil {
	//				//已阻止。我们可以从BlockError中获取阻塞原因
	//				time.Sleep(time.Duration(rand.Uint64()%2) * time.Millisecond)
	//			} else {
	//				// 通过
	//				time.Sleep(time.Duration(rand.Uint64()%2) * time.Millisecond)
	//				e.Exit()
	//			}
	//		}
	//	}()
	//}
	//
	//go func() {
	//	time.Sleep(time.Second * 5)
	//	// 模拟内存使用量为1536字节，因此QPS阈值应为550
	//	system_metric.SetSystemMemoryUsage(1536)
	//	fmt.Println("内存使用量为1536:", new(fitutil.ParseTime).HSM(time.Now().Unix()))
	//
	//	time.Sleep(time.Second * 5)
	//	// 模拟内存使用量为1536字节，因此QPS阈值应为100
	//	system_metric.SetSystemMemoryUsage(2048)
	//	fmt.Println("内存使用量为2048:", new(fitutil.ParseTime).HSM(time.Now().Unix()))
	//
	//	time.Sleep(time.Second * 5)
	//	// mock memory usage is 1536 bytes, so QPS threshold should be 100
	//	system_metric.SetSystemMemoryUsage(100000)
	//	fmt.Println("内存使用量为100000:", new(fitutil.ParseTime).HSM(time.Now().Unix()))
	//	time.Sleep(time.Second * 5)
	//	ch <- true
	//}()
	//
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
