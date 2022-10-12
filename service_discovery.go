package fit

import (
	"context"
	"go.etcd.io/etcd/client/v3"
	"math/rand"
	"time"
)

type LoadBalancingPolicy struct {
	Addrs    []string
	CurIndex int
}

func NewLoadBalancing() *LoadBalancingPolicy {
	return &LoadBalancingPolicy{}
}

func (l *LoadBalancingPolicy) Add(addr string) {
	l.Addrs = append(l.Addrs, addr)
}

// SelectByRand random
func (l *LoadBalancingPolicy) SelectByRand() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	if len(l.Addrs) == 0 {
		return ""
	}
	index := r.Intn(len(l.Addrs))
	return l.Addrs[index]
}

func NewServiceDiscovery(ctx context.Context, client *clientv3.Client, prefix string) (*LoadBalancingPolicy, error) {
	result, err := client.Get(ctx, prefix, []clientv3.OpOption{clientv3.WithPrefix()}...)
	if err != nil {
		return nil, err
	}

	var l LoadBalancingPolicy
	for _, v := range result.Kvs {
		l.Add(string(v.Value))
	}
	return &l, nil
}
