package leastconnbalance

import (
	"math"
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

const Name = "least_conn_balance"

func newBuilder() balancer.Builder {
	return base.NewBalancerBuilder(Name, &leastConnPickerBuilder{}, base.Config{HealthCheck: true})
}

func init() {
	balancer.Register(newBuilder())
}

type leastConnPickerBuilder struct{}

func (*leastConnPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}

	scs := make([]balancer.SubConn, 0, len(info.ReadySCs))
	for sc := range info.ReadySCs {
		scs = append(scs, sc)
	}

	return &leastConnPicker{
		subConns:   scs,
		connCounts: make(map[balancer.SubConn]int64),
	}
}

type leastConnPicker struct {
	subConns   []balancer.SubConn
	connCounts map[balancer.SubConn]int64
	mu         sync.Mutex
}

func (p *leastConnPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.subConns) == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	// 找到连接数最少的服务实例
	var selected balancer.SubConn
	minConns := int64(math.MaxInt64)

	for _, sc := range p.subConns {
		count := p.connCounts[sc]
		if count < minConns {
			minConns = count
			selected = sc
		}
	}

	// 如果没有找到合适的连接，选择第一个
	if selected == nil {
		selected = p.subConns[0]
	}

	// 增加连接计数
	p.connCounts[selected]++

	return balancer.PickResult{
		SubConn: selected,
		Done: func(info balancer.DoneInfo) {
			// 请求完成后减少连接计数
			p.mu.Lock()
			if p.connCounts[selected] > 0 {
				p.connCounts[selected]--
			}
			p.mu.Unlock()
		},
	}, nil
}
