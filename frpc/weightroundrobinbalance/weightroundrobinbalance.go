package weightroundrobinbalance

import (
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

const Name = "weight_round_robin_balance"

func init() {
	balancer.Register(newBuilder())
}

func newBuilder() balancer.Builder {
	return base.NewBalancerBuilder(Name, &rrPickerBuilder{}, base.Config{HealthCheck: true})
}

type rrPickerBuilder struct{}

func (*rrPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}

	wss := make([]*WeightedServer, 0, len(info.ReadySCs))
	for sc, scInfo := range info.ReadySCs {
		var weight float64
		if scInfo.Address.Attributes != nil {
			if w, ok := scInfo.Address.Attributes.Value("weight").(float64); ok {
				weight = w
			}
		}

		wss = append(wss, &WeightedServer{
			conn:    sc,
			Weight:  weight,
			Current: 0,
		})
	}

	return &rrPicker{
		wss: wss,
	}
}

type WeightedServer struct {
	conn     balancer.SubConn
	connInfo base.SubConnInfo
	Weight   float64
	Current  float64
}

type rrPicker struct {
	wss []*WeightedServer

	mu sync.Mutex
}

func (p *rrPicker) Pick(balancer.PickInfo) (balancer.PickResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var best *WeightedServer
	var total float64

	for _, ws := range p.wss {
		total += ws.Weight
		ws.Current += ws.Weight
		if best == nil || ws.Current > best.Current {
			best = ws
		}
	}

	if best == nil {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	best.Current -= total

	return balancer.PickResult{SubConn: best.conn}, nil
}
