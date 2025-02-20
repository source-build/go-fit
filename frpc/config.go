package frpc

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"os"
)

type TLSType string

const (
	// TLSTypeOneWay TLS one-way authentication
	TLSTypeOneWay TLSType = "one-way"

	// TLSTypeMTLS TLS mutual authentication
	TLSTypeMTLS TLSType = "mTLS"
)

// A RpcClientConf is a rpc client config.
type RpcClientConf struct {
	Namespace string

	EtcdClient *clientv3.Client

	TLSType TLSType

	CertFile string

	ServerNameOverride string

	KeyFile string

	CAFile string
}

func (r RpcClientConf) GetNamespace() string {
	if r.Namespace == "" {
		return "default"
	}

	return r.Namespace
}

func (r RpcClientConf) clientTLS() (grpc.DialOption, error) {
	if r.CertFile == "" || r.ServerNameOverride == "" {
		return nil, errors.New("when using TLS authentication, please ensure that the Cert file 'ServerNameOverride' content is provided during initialization")
	}

	cred, err := credentials.NewClientTLSFromFile(r.CertFile, r.ServerNameOverride)
	if err != nil {
		return nil, err
	}

	return grpc.WithTransportCredentials(cred), nil
}

func (r RpcClientConf) clientmTLS() (grpc.DialOption, error) {
	if r.CertFile == "" || r.KeyFile == "" {
		return nil, errors.New("the Cert file and Key file cannot be empty. You should pass in the corresponding file path during initialization")
	}

	clientCert, err := tls.LoadX509KeyPair(r.CertFile, r.KeyFile)
	if err != nil {
		return nil, err
	}

	if r.CAFile == "" {
		return nil, errors.New("the CA file cannot be empty. You should pass in the CA file path during initialization")
	}

	caCert, err := os.ReadFile(r.CAFile)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()

	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("failed to append ca certs")
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		ServerName:   r.ServerNameOverride,
		RootCAs:      certPool,
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(config)), nil
}
