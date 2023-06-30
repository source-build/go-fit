package fit

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/avast/retry-go/v4"
	"go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/resolver"
	"io/ioutil"
	"time"
)

//var tls credentials.TransportCredentials
var scheme string
var creds credentials.TransportCredentials

type Config struct {
	rule        string
	scheme      string
	attempts    uint
	notTimeout  bool
	timeout     time.Duration
	ctx         context.Context
	cancel      context.CancelFunc
	dialOptions []grpc.DialOption
}

type Option func(*Config)

type GrpcBuilderConfig struct {
	EtcdClient         *clientv3.Client
	ClientCertPath     string
	ClientKeyPath      string
	RootCrtPath        string
	ServerNameOverride string
}

// NewGrpcClientBuilder create a new grpc client parser
//
// r parameter is the name of fusing rule
// Pass in the second parameter to enable interface protection.
// Please ensure that the rules have been InitSentinel and loaded LoadBreakerRule successfully
func NewGrpcClientBuilder(g GrpcBuilderConfig) error {
	builder := &Builder{
		Client: g.EtcdClient,
	}
	scheme = builder.Scheme()
	resolver.Register(builder)

	newClientTls, err := NewClientTLS(&CertPool{
		CertFile:   g.ClientCertPath,
		KeyFile:    g.ClientKeyPath,
		CaCert:     g.RootCrtPath,
		ServerName: g.ServerNameOverride,
	})
	if err != nil {
		return err
	}

	creds = newClientTls
	return nil
}

// GrpcDial gRPC client
//
// Please call NewDefaultBuilder or NewBuilder before calling this function
func GrpcDial(serveName string, opts ...Option) (*grpc.ClientConn, error) {
	if creds == nil {
		return nil, errors.New("first, please call 'NewGrpcClientBuilder'")
	}
	config := &Config{}
	for _, opt := range opts {
		opt(config)
	}
	target := scheme + "://" + serveName
	defaultDialOption(config)

	if len(config.rule) > 0 {
		var conn *grpc.ClientConn
		var err error
		e, b := sentinel.Entry(config.rule)
		if b != nil {
			return nil, errors.New("failed to establish connection. The failure reason may be external service error")
		} else {
			conn, err = grpc.Dial(target, config.dialOptions...)
			if err != nil {
				sentinel.TraceError(e, err)
			}
			e.Exit()
		}
		return conn, err
	}

	return grpc.Dial(target, config.dialOptions...)
}

func GrpcDialContext(serveName string, opts ...Option) (*grpc.ClientConn, error) {
	if creds == nil {
		return nil, errors.New("first, please call 'NewGrpcClientBuilder'")
	}
	config := &Config{}
	for _, opt := range opts {
		opt(config)
	}
	target := scheme + "://" + serveName
	defaultDialOption(config)
	config.dialOptions = append(config.dialOptions, grpc.WithBlock())

	if config.timeout == 0 {
		config.timeout = time.Second * 10
	}

	if config.ctx == nil {
		config.ctx = context.Background()
	}

	if !config.notTimeout {
		config.ctx, config.cancel = context.WithTimeout(config.ctx, config.timeout)
		defer config.cancel()
	}

	if len(config.rule) > 0 {
		var conn *grpc.ClientConn
		var err error
		e, b := sentinel.Entry(config.rule)
		if b != nil {
			return nil, errors.New("failed to establish connection. The failure reason may be external service error")
		} else {
			conn, err = grpc.DialContext(config.ctx, target, config.dialOptions...)
			if err != nil {
				sentinel.TraceError(e, err)
			}
			e.Exit()
		}
		return conn, err
	}

	if config.attempts > 1 {
		var conn *grpc.ClientConn
		ctx, cancel := context.WithTimeout(config.ctx, config.timeout)
		err := retry.Do(func() error {
			defer cancel()
			c, err := grpc.DialContext(ctx, target, config.dialOptions...)
			if err != nil {
				return err
			}
			conn = c
			return nil
		},
			retry.Attempts(config.attempts),
			retry.DelayType(func(n uint, err error, c *retry.Config) time.Duration {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), config.timeout)
				return retry.BackOffDelay(n, err, c)
			}),
		)
		return conn, err
	}

	return grpc.DialContext(config.ctx, target, config.dialOptions...)
}

func CloseGrpc(conn *grpc.ClientConn) {
	if err := conn.Close(); err != nil {
		Error("info", "gRPC dial close failed!", "err", err)
	}
}

func defaultDialOption(opt *Config) {
	opt.dialOptions = append(opt.dialOptions, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	opt.dialOptions = append(opt.dialOptions, grpc.WithTransportCredentials(creds))
}

func Rule(name string) Option {
	return func(c *Config) {
		c.rule = name
	}
}

func Attempts(u uint) Option {
	return func(c *Config) {
		c.attempts = u
	}
}

func DialOption(opts ...grpc.DialOption) Option {
	return func(c *Config) {
		c.dialOptions = opts
	}
}

func WithContext() Option {
	return func(c *Config) {
		c.dialOptions = append(c.dialOptions, grpc.WithUnaryInterceptor(WithGrpcCtx()))
	}
}

func Context(ctx context.Context) Option {
	return func(c *Config) {
		c.ctx = ctx
	}
}

func NotTimeout() Option {
	return func(c *Config) {
		c.notTimeout = true
	}
}

func WithTimeout(t time.Duration) Option {
	return func(c *Config) {
		c.timeout = t
	}
}

func SetScheme(v string) {
	if len(v) > 0 {
		scheme = v
	}
}

func SchemeJoin(name string) string {
	return scheme + "://" + name
}

type CertPool struct {
	CertFile   string
	KeyFile    string
	CaCert     string
	ServerName string
}

func newTls(c *CertPool, env string) (credentials.TransportCredentials, error) {
	if len(c.CertFile) == 0 {
		return nil, errors.New("certFile Cannot be empty")
	}
	if len(c.KeyFile) == 0 {
		return nil, errors.New("KeyFile Cannot be empty")
	}
	if len(c.CaCert) == 0 {
		return nil, errors.New("CaCert Cannot be empty")
	}

	pair, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(c.CaCert)
	if err != nil {
		return nil, err
	}

	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		return nil, err
	}

	var cred credentials.TransportCredentials
	if env == "server" {
		cred = credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{pair},
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ClientCAs:    certPool,
		})
	}
	if env == "client" {
		cred = credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{pair},
			ServerName:   c.ServerName,
			RootCAs:      certPool,
		})
	}

	return cred, nil
}

func NewServiceTLS(c *CertPool) (credentials.TransportCredentials, error) {
	return newTls(c, "server")
}

func NewClientTLS(c *CertPool) (credentials.TransportCredentials, error) {
	return newTls(c, "client")
}
