package frpc

import (
	"context"
	"errors"
	"fmt"
	"github.com/source-build/go-fit/frpc/randombalance"
	"github.com/source-build/go-fit/frpc/weightroundrobinbalance"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

type dialOptions struct {
	gOpts        []grpc.DialOption
	ctx          context.Context
	isDisableTLS bool
	cancel       context.CancelFunc
	isBlock      bool
	scheme       string // default etcd
}

type DialOption interface {
	apply(*dialOptions)
}

// funcDialOption wraps a function that modifies dialOptions into an
// implementation of the DialOption interface.
type funcDialOption struct {
	f func(*dialOptions)
}

func (fdo *funcDialOption) apply(do *dialOptions) {
	fdo.f(do)
}

func newFuncDialOption(f func(*dialOptions)) *funcDialOption {
	return &funcDialOption{
		f: f,
	}
}

// Return a default option, where the grpc load balancer defaults to polling mode
func defaultDialOptions() dialOptions {
	return dialOptions{
		scheme: EtcdScheme,
	}
}

var rpcClientConf RpcClientConf

func Init(opt RpcClientConf) error {
	rpcClientConf = opt
	registerResolver(rpcClientConf)
	return nil
}

func NewClient(target string, opts ...DialOption) (conn *grpc.ClientConn, err error) {
	option := defaultDialOptions()
	for _, o := range opts {
		o.apply(&option)
	}

	if rpcClientConf.TLSType == TLSTypeOneWay {
		if err = option.tlsOneWayHandler(); err != nil {
			return nil, err
		}
	}

	if rpcClientConf.TLSType == TLSTypeMTLS {
		if err = option.mTLSHandler(); err != nil {
			return nil, err
		}
	}

	if err = option.checkScheme(); err != nil {
		return nil, err
	}

	target = option.buildTarget(target)

	ctx := context.Background()

	if option.cancel != nil {
		defer option.cancel()
		if option.isBlock {
			ctx = option.ctx
		}
	}

	return grpc.DialContext(ctx, target, option.gOpts...)
}

func (d *dialOptions) buildTarget(target string) string {
	if d.scheme == EtcdScheme {
		return BuildEtcdTarget(target)
	}

	return ""
}

func (d *dialOptions) checkScheme() error {
	if d.scheme == EtcdScheme && rpcClientConf.EtcdClient == nil {
		return errors.New("unable to find etcd connection information, please ensure that an etcd client already exists")
	}

	return nil
}

func (d *dialOptions) tlsOneWayHandler() error {
	if d.isDisableTLS {
		return nil
	}

	tls, err := rpcClientConf.clientTLS()
	if err != nil {
		return err
	}

	d.gOpts = append(d.gOpts, tls)

	return nil
}

func (d *dialOptions) mTLSHandler() error {
	if d.isDisableTLS {
		return nil
	}

	tls, err := rpcClientConf.clientmTLS()
	if err != nil {
		return err
	}

	d.gOpts = append(d.gOpts, tls)

	return nil
}

// WithBalancerRandom Using a load balancer with random load balancing
func WithBalancerRandom(opt ...grpc.DialOption) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.gOpts = append(o.gOpts, grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, randombalance.Name)))
	})
}

// WithBalancerRoundRobin defines a roundRobin balancer. roundRobin balancer is
func WithBalancerRoundRobin(opt ...grpc.DialOption) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.gOpts = append(o.gOpts, grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, roundrobin.Name)))
	})
}

// WithBalancerWeightRoundRobin Using weighted polling scheme as load balancing
func WithBalancerWeightRoundRobin(opt ...grpc.DialOption) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.gOpts = append(o.gOpts, grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, weightroundrobinbalance.Name)))
	})
}

// WithGrpcOption receive grpc.DialOption
func WithGrpcOption(opt ...grpc.DialOption) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.gOpts = append(o.gOpts, opt...)
	})
}

// WithBlock returns a DialOption which makes callers of Dial block until the underlying connection is up. Without this, Dial returns immediately and connecting the server happens in background.
func WithBlock() DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.isBlock = true
		o.gOpts = append(o.gOpts, grpc.WithBlock())
	})
}

func DisableTLS() DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.isDisableTLS = true
	})
}

func WithCtx(ctx context.Context) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.ctx = ctx
	})
}

// WithTimeoutCtx Set a timeout context and receive a parameter with a default timeout of 10 seconds.
// This setting is only effective when WithBlock is used.
func WithTimeoutCtx(t ...time.Duration) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		var timeout time.Duration
		if len(t) > 0 {
			timeout = t[0]
		} else {
			timeout = time.Second * 10
		}

		o.ctx, o.cancel = context.WithTimeout(context.Background(), timeout)
	})
}

func NewDirectClient(target string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	return grpc.DialContext(context.Background(), target, opts...)
}

// IsNotFoundServiceErr Determine if the error is due to the lack of available services
func IsNotFoundServiceErr(err error) bool {
	return status.Convert(err).Code() == codes.Unavailable
}
