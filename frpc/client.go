package frpc

import (
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type dialOptions struct {
	gOpts        []grpc.DialOption
	isDisableTLS bool
	scheme       string
}

type DialOptions interface {
	apply(*dialOptions)
	identifier() string
}

// wraps a function that modifies dialOptions into an
// implementation of the DialOption interface.
type identifiableDialOption struct {
	fn func(*dialOptions)
	id string
}

func (i *identifiableDialOption) apply(do *dialOptions) {
	i.fn(do)
}

func (i *identifiableDialOption) identifier() string {
	return i.id
}

// Return a default option, where the grpc load balancer defaults to round robin mode
func defaultDialOptions() dialOptions {
	gOpts := []grpc.DialOption{grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, roundrobin.Name))}

	// Add otelgrpc StatsHandler if tracing is enabled
	if rpcClientConf.EnableTrace {
		gOpts = append(gOpts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	}

	return dialOptions{
		scheme: EtcdScheme,
		gOpts:  gOpts,
	}
}

var rpcClientConf RpcClientConf

func Init(opt RpcClientConf) error {
	rpcClientConf = opt
	registerEtcdResolver(rpcClientConf)

	poolConfig := PoolConfig{}

	if opt.PoolConfig != nil {
		poolConfig = *opt.PoolConfig
	}

	InitPool(poolConfig)

	return nil
}

func NewClient(target string, opts ...DialOptions) (poolConn *PooledConn, err error) {
	var builder strings.Builder
	builder.WriteString(target)
	if len(opts) > 0 {
		builder.WriteString(":")

		for i, o := range opts {
			builder.WriteString(o.identifier())
			if i+1 < len(opts) {
				builder.WriteString(",")
			}
		}
	}

	connId := builder.String()

	createFunc := func() (*grpc.ClientConn, error) {
		option := defaultDialOptions()
		for _, o := range opts {
			o.apply(&option)
		}

		if err = option.authHandler(); err != nil {
			return nil, err
		}

		if err = option.checkScheme(); err != nil {
			return nil, err
		}

		return grpc.NewClient(option.target(target), option.gOpts...)
	}

	return GetOrCreatePoolConnection(connId, createFunc)
}

func (d *dialOptions) authHandler() error {
	// Set transport layer credentials
	transportCred, err := rpcClientConf.clientTransportCredentials()
	if err != nil {
		return fmt.Errorf("failed to setup transport credentials: %w", err)
	}

	d.gOpts = append(d.gOpts, transportCred)

	// If Token authentication is configured, add Per RPC credentials (optional)
	if rpcClientConf.hasToken() {
		tokenCred, err := rpcClientConf.clientToken()
		if err != nil {
			return fmt.Errorf("failed to setup token credentials: %w", err)
		}
		d.gOpts = append(d.gOpts, tokenCred)
	}

	return nil
}

func (d *dialOptions) target(target string) string {
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

// WithGrpcOption receive grpc.DialOption
func WithGrpcOption(opts ...grpc.DialOption) DialOptions {
	return &identifiableDialOption{
		fn: func(o *dialOptions) {
			o.gOpts = append(o.gOpts, opts...)
		},
		id: fmt.Sprintf("g_opt:%d", len(opts)),
	}
}

func NewDirectClient(target string, opts ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	return grpc.NewClient(target, opts...)
}

// IsNotFoundServiceErr Determine if the error is due to the lack of available services
func IsNotFoundServiceErr(err error) bool {
	return status.Convert(err).Code() == codes.Unavailable
}
