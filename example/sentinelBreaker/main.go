package main

import (
	"errors"
	"fmt"
	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/config"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/source-build/go-fit"
	"log"
	"math/rand"
	"time"
)

var breakerRules = []*circuitbreaker.Rule{
	// 慢调用比例规则
	{
		Resource:         "abc",
		Strategy:         circuitbreaker.SlowRequestRatio, //慢调用比例策略。熔断策略，目前支持SlowRequestRatio、ErrorRatio、ErrorCount三种；
		RetryTimeoutMs:   3000,                            //熔断触发后持续的时间（单位为 ms）。资源进入熔断状态后，在配置的熔断时长内，请求都会快速失败。熔断结束后进入探测恢复模式（HALF-OPEN）
		MinRequestAmount: 10,                              //静默数量，若当前统计周期内的请求数小于此值，即使达到熔断条件规则也不会触发。
		StatIntervalMs:   5000,                            //统计的时间窗口长度（单位为 ms）
		MaxAllowedRtMs:   50,                              //仅对慢调用熔断策略生效，MaxAllowedRtMs 是判断请求是否是慢调用的临界值，也就是如果请求的response time小于或等于MaxAllowedRtMs，那么就不是慢调用；如果response time大于MaxAllowedRtMs，那么当前请求就属于慢调用。
		Threshold:        0.5,                             //对于错误比例策略，Threshold表示的是错误比例的阈值(小数表示，比如0.1表示10%)。
	},
	// 错误比例规则,统计周期内资源请求访问异常的比例大于设定的阈值，则接下来的熔断周期内对资源的访问会自动地被熔断
	{
		Resource:         "errorRatio",
		Strategy:         circuitbreaker.ErrorRatio,
		RetryTimeoutMs:   3000, //熔断触发后持续的时间（单位为 ms）
		MinRequestAmount: 10,   //静默请求数
		StatIntervalMs:   5000, //统计周期
		Threshold:        0.4,  //错误比例的阈值(小数表示，比如0.1表示10%)
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
	go func() {
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
	go func() {
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
