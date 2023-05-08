package fit

import (
	"context"
	"encoding/json"
	"errors"
	"go.etcd.io/etcd/client/v3"
	"math/rand"
	"time"
)

type LoadBalancingPolicy struct {
	Services []RegisterCenterValue
	Desc     string
}

func NewLoadBalancing() *LoadBalancingPolicy {
	return &LoadBalancingPolicy{}
}

func (l *LoadBalancingPolicy) Add(s RegisterCenterValue) {
	l.Services = append(l.Services, s)
}

// SelectByRand random
func (l *LoadBalancingPolicy) SelectByRand() (RegisterCenterValue, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	if len(l.Services) == 0 {
		return RegisterCenterValue{}, errors.New(l.Desc)
	}
	index := r.Intn(len(l.Services))
	return l.Services[index], nil
}

func NewServiceDiscovery(ctx context.Context, client *clientv3.Client, prefix string) (*LoadBalancingPolicy, error) {
	result, err := client.Get(ctx, prefix, []clientv3.OpOption{clientv3.WithPrefix()}...)
	if err != nil {
		return nil, err
	}

	var l LoadBalancingPolicy
	for _, v := range result.Kvs {
		var rcv RegisterCenterValue
		if err := json.Unmarshal(v.Value, &rcv); err == nil {
			if rcv.Status == ServiceStatusRun {
				l.Add(rcv)
			} else if rcv.Reason != "" {
				l.Desc = rcv.Reason
			}
		}
	}
	if len(l.Services) == 0 && l.Desc == "" {
		l.Desc = "找不到可用的节点"
	}
	return &l, nil
}
