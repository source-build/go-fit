package fit

import (
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
)

//var tls credentials.TransportCredentials
var scheme string
var creds credentials.TransportCredentials

type Config struct {
	rule     string
	scheme   string
	attempts uint
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

	tls, err := NewClientTLS(&CertPool{
		CertFile:   g.ClientCertPath,
		KeyFile:    g.ClientKeyPath,
		CaCert:     g.RootCrtPath,
		ServerName: g.ServerNameOverride,
	})
	if err != nil {
		return err
	}

	creds = tls
	return nil
}

// GrpcDial gRPC client
//
// Please call NewDefaultBuilder or NewBuilder before calling this function
func GrpcDial(serveName string, opts ...Option) (*grpc.ClientConn, error) {
	if creds == nil {
		return nil, errors.New("first, please call NewGrpcClientBuilder()")
	}
	config := &Config{}
	for _, opt := range opts {
		opt(config)
	}
	target := scheme + "://" + serveName

	if len(config.rule) > 0 {
		var conn *grpc.ClientConn
		e, b := sentinel.Entry(config.rule)
		if b != nil {
			return nil, errors.New("failed to establish connection. The failure reason may be external service error")
		} else {
			cc, err := grpc.Dial(
				target,
				grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
				grpc.WithTransportCredentials(creds),
			)
			if err != nil {
				sentinel.TraceError(e, err)
			}
			conn = cc
			e.Exit()
		}
		return conn, nil
	}

	if config.attempts > 1 {
		var conn *grpc.ClientConn
		err := retry.Do(
			func() error {
				c, err := grpc.Dial(
					target,
					grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
					grpc.WithTransportCredentials(creds),
				)
				if err != nil {
					return err
				}
				conn = c
				return nil
			},
			retry.Attempts(config.attempts), //最大重试次数
		)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}

	conn, err := grpc.Dial(
		target,
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		return nil, err
	}
	return conn, nil
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
