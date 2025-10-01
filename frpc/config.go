// Package frpc provides a high-performance gRPC client connection pool with advanced features
// including service discovery, load balancing, and automatic connection management.
//
// The connection pool is designed to handle high-concurrency scenarios by reusing connections
// and implementing intelligent load balancing algorithms. It supports multiple transport
// security types including insecure, TLS one-way, and mutual TLS authentication.
//
// Key Features:
//   - Multi-connection pooling per service
//   - Automatic connection health monitoring
//   - Configurable load balancing strategies
//   - Service discovery integration with etcd
//   - Graceful connection cleanup and lifecycle management
//   - Thread-safe operations with minimal lock contention
//
// Basic Usage:
//
//	// Initialize the connection pool
//	err := frpc.Init(frpc.RpcClientConf{
//		EtcdClient: etcdClient,
//		Namespace:  "production",
//		TransportType: frpc.TransportTypeMTLS,
//		CertFile: "client.crt",
//		KeyFile:  "client.key",
//		CAFile:   "ca.crt",
//	})
//
//	// Get a connection from the pool
//	client, err := frpc.NewClient("user-service")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close() // Important: always close to return connection to pool
//
//	// Use the connection for gRPC calls
//	userClient := pb.NewUserServiceClient(client)
//	response, err := userClient.GetUser(ctx, &pb.GetUserRequest{ID: "123"})

package frpc

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// TransportType defines the transport layer security type for gRPC connections.
// It determines how the client authenticates with the server and what level
// of encryption is applied to the communication channel.
type TransportType string

const (
	// TransportTypeInsecure uses plaintext communication without any encryption.
	// This should only be used in development environments or internal networks
	// where security is not a concern. Not recommended for production use.
	TransportTypeInsecure TransportType = "insecure"

	// TransportTypeOneWay enables TLS with server-side certificate verification.
	// The client verifies the server's identity using the provided certificate,
	// but the server does not verify the client's identity. This is suitable
	// for most client-server scenarios where client authentication is handled
	// at the application level.
	TransportTypeOneWay TransportType = "one-way"

	// TransportTypeMTLS enables mutual TLS authentication where both client
	// and server verify each other's certificates. This provides the highest
	// level of security and is recommended for production environments where
	// strong authentication is required.
	TransportTypeMTLS TransportType = "mTLS"
)

// RpcClientConf contains all configuration parameters for initializing the gRPC client
// connection pool. This configuration is used globally and affects all connections
// created through the pool.
//
// Security Configuration:
// The transport security can be configured using TransportType along with the
// appropriate certificate files. For production environments, mTLS is recommended.
//
// Service Discovery:
// When EtcdClient is provided, the connection pool will use etcd for service
// discovery, automatically discovering and connecting to available service instances.
//
// Connection Pooling:
// The PoolConfig parameter allows fine-tuning of connection pool behavior,
// including connection limits, idle timeouts, and cleanup intervals.
type RpcClientConf struct {
	// Namespace provides service isolation within etcd. Services registered
	// under different namespaces are completely isolated from each other.
	// If empty, defaults to "default".
	Namespace string

	// PoolConfig contains connection pool specific settings such as maximum
	// idle time, cleanup intervals, and connection limits per service.
	// If nil, default values will be used.
	PoolConfig *PoolConfig

	// EtcdClient is required for service discovery functionality. When provided,
	// the connection pool will automatically discover service instances registered
	// in etcd and distribute load across them.
	EtcdClient *clientv3.Client

	// TransportType specifies the security level for gRPC connections.
	// Must be one of: TransportTypeInsecure, TransportTypeOneWay, or TransportTypeMTLS.
	TransportType TransportType

	// TokenCredentials provides per-RPC authentication credentials.
	// This is optional and can be used in conjunction with transport security
	// for additional authentication layers.
	TokenCredentials credentials.PerRPCCredentials

	// CertFile path to the client certificate file (PEM format).
	// Required for TransportTypeOneWay and TransportTypeMTLS.
	CertFile string

	// ServerNameOverride specifies the server name to verify in the certificate.
	// This is required for TLS connections and must match the server's certificate.
	ServerNameOverride string

	// KeyFile path to the client private key file (PEM format).
	// Required for TransportTypeMTLS authentication.
	KeyFile string

	// CAFile path to the Certificate Authority file (PEM format).
	// Required for TransportTypeMTLS to verify the server's certificate.
	CAFile string
}

// GetNamespace returns the configured namespace, defaulting to "default" if empty.
// This method ensures that a valid namespace is always returned for service isolation.
func (r RpcClientConf) GetNamespace() string {
	if r.Namespace == "" {
		return "default"
	}
	return r.Namespace
}

// clientTransportCredentials creates the appropriate transport credentials based on
// the configured TransportType. This method handles the complexity of setting up
// different security configurations and returns a gRPC dial option.
//
// Returns:
//   - grpc.DialOption: The transport credentials dial option
//   - error: Any error encountered during credential setup
//
// Security Levels:
//   - Insecure: No encryption, plaintext communication
//   - OneWay: TLS with server certificate verification
//   - mTLS: Mutual TLS with both client and server certificate verification
func (r RpcClientConf) clientTransportCredentials() (grpc.DialOption, error) {
	switch r.TransportType {
	case TransportTypeInsecure:
		return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
	case TransportTypeOneWay:
		return r.clientTLS()
	case TransportTypeMTLS:
		return r.clientMTLS()
	default:
		return nil, errors.New("unsupported transport type")
	}
}

// clientTLS configures one-way TLS authentication where only the server's
// certificate is verified. This method requires CertFile and ServerNameOverride
// to be properly configured.
//
// Returns:
//   - grpc.DialOption: TLS credentials dial option
//   - error: Configuration or certificate loading error
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

// clientMTLS configures mutual TLS authentication where both client and server
// certificates are verified. This provides the highest level of security and
// requires proper certificate infrastructure.
//
// Certificate Requirements:
//   - CertFile: Client certificate in PEM format
//   - KeyFile: Client private key in PEM format
//   - CAFile: Certificate Authority for server verification
//   - ServerNameOverride: Expected server name in certificate
//
// Returns:
//   - grpc.DialOption: mTLS credentials dial option
//   - error: Configuration or certificate loading error
func (r RpcClientConf) clientMTLS() (grpc.DialOption, error) {
	if r.CertFile == "" || r.KeyFile == "" {
		return nil, errors.New("the Cert file and Key file cannot be empty. You should pass in the corresponding file path during initialization")
	}

	// Load client certificate and private key
	clientCert, err := tls.LoadX509KeyPair(r.CertFile, r.KeyFile)
	if err != nil {
		return nil, err
	}

	if r.CAFile == "" {
		return nil, errors.New("the CA file cannot be empty. You should pass in the CA file path during initialization")
	}

	// Load CA certificate for server verification
	caCert, err := os.ReadFile(r.CAFile)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("failed to append ca certs")
	}

	// Configure TLS with mutual authentication
	config := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		ServerName:   r.ServerNameOverride,
		RootCAs:      certPool,
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(config)), nil
}

// clientToken creates per-RPC credentials for token-based authentication.
// This can be used in addition to transport security for application-level
// authentication such as JWT tokens or API keys.
//
// Returns:
//   - grpc.DialOption: Per-RPC credentials dial option
//   - error: Configuration error if TokenCredentials is nil
func (r RpcClientConf) clientToken() (grpc.DialOption, error) {
	if r.TokenCredentials == nil {
		return nil, errors.New("when using Token authentication, please ensure that TokenCredentials is provided during initialization")
	}

	return grpc.WithPerRPCCredentials(r.TokenCredentials), nil
}

// hasToken returns true if token-based authentication is configured.
// This is used internally to determine whether to apply per-RPC credentials.
func (r RpcClientConf) hasToken() bool {
	return r.TokenCredentials != nil
}
