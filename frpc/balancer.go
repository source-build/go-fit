package frpc

import (
	"fmt"

	"github.com/source-build/go-fit/frpc/leastconnbalance"
	"github.com/source-build/go-fit/frpc/randombalance"
	"github.com/source-build/go-fit/frpc/weightroundrobinbalance"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
)

// WithBalancerRandom Using a load balancer with random load balancing
func WithBalancerRandom() DialOptions {
	return &identifiableDialOption{
		fn: func(o *dialOptions) {
			o.gOpts = append(o.gOpts, grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, randombalance.Name)))
		},
		id: randombalance.Name,
	}
}

// WithBalancerRoundRobin defines a roundRobin balancer. roundRobin balancer is
func WithBalancerRoundRobin() DialOptions {
	return &identifiableDialOption{
		fn: func(o *dialOptions) {
			o.gOpts = append(o.gOpts, grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, roundrobin.Name)))
		},
		id: roundrobin.Name,
	}
}

// WithBalancerWeightRoundRobin Using weighted polling scheme as load balancing
func WithBalancerWeightRoundRobin() DialOptions {
	return &identifiableDialOption{
		fn: func(o *dialOptions) {
			o.gOpts = append(o.gOpts, grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, weightroundrobinbalance.Name)))
		},
		id: weightroundrobinbalance.Name,
	}
}

// WithBalancerLeastConn Using least connections load balancing
func WithBalancerLeastConn() DialOptions {
	return &identifiableDialOption{
		fn: func(o *dialOptions) {
			o.gOpts = append(o.gOpts, grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, leastconnbalance.Name)))
		},
		id: leastconnbalance.Name,
	}
}
