package main

import (
	"fmt"
	"github.com/avast/retry-go/v4"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	url := "http://example11.com"
	var body []byte

	err := retry.Do(
		func() error {
			resp, err := http.Get(url)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			body, err = io.ReadAll(resp.Body)
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
