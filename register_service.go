package fit

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"path"
	"strconv"
	"strings"
	"time"
)

type EtcdConfig struct {
	// Endpoints is a list of URLs.
	Endpoints []string `json:"endpoints"`

	// AutoSyncInterval is the interval to update endpoints with its latest members.
	// 0 disables auto-sync. By default auto-sync is disabled.
	AutoSyncInterval time.Duration `json:"auto-sync-interval"`

	// DialTimeout is the timeout for failing to establish a connection.
	DialTimeout time.Duration `json:"dial-timeout"`

	// DialKeepAliveTime is the time after which client pings the server to see if
	// transport is alive.
	DialKeepAliveTime time.Duration `json:"dial-keep-alive-time"`

	// DialKeepAliveTimeout is the time that the client waits for a response for the
	// keep-alive probe. If the response is not received in this time, the connection is closed.
	DialKeepAliveTimeout time.Duration `json:"dial-keep-alive-timeout"`

	// MaxCallSendMsgSize is the client-side request send limit in bytes.
	// If 0, it defaults to 2.0 MiB (2 * 1024 * 1024).
	// Make sure that "MaxCallSendMsgSize" < server-side default send/recv limit.
	// ("--max-request-bytes" flag to etcd or "embed.Config.MaxRequestBytes").
	MaxCallSendMsgSize int

	// MaxCallRecvMsgSize is the client-side response receive limit.
	// If 0, it defaults to "math.MaxInt32", because range response can
	// easily exceed request send limits.
	// Make sure that "MaxCallRecvMsgSize" >= server-side default send/recv limit.
	// ("--max-request-bytes" flag to etcd or "embed.Config.MaxRequestBytes").
	MaxCallRecvMsgSize int

	// TLS holds the client secure credentials, if any.
	TLS *tls.Config

	// Username is a user name for authentication.
	Username string `json:"username"`

	// Password is a password for authentication.
	Password string `json:"password"`

	// RejectOldCluster when set will refuse to create a client against an outdated cluster.
	RejectOldCluster bool `json:"reject-old-cluster"`

	// DialOptions is a list of dial options for the grpc client (e.g., for interceptors).
	// For example, pass "grpc.WithBlock()" to block until the underlying connection is up.
	// Without this, Dial returns immediately and connecting the server happens in background.
	DialOptions []grpc.DialOption

	// Context is the default client context; it can be used to cancel grpc dial out and
	// other operations that do not have an explicit context.
	Context context.Context

	// Logger sets client-side logger.
	// If nil, fallback to building LogConfig.
	Logger *zap.Logger

	// LogConfig configures client-side logger.
	// If nil, use the default logger.
	// TODO: configure gRPC logger
	LogConfig *zap.Config

	// PermitWithoutStream when set will allow client to send keepalive pings to server without any active streams(RPCs).
	PermitWithoutStream bool `json:"permit-without-stream"`

	// TODO: support custom balancer picker
}

type RegisterValue struct {
	Timestamp int64 `json:"timestamp"`

	IP string `json:"ip"`

	Port string `json:"port"`

	Meta H `json:"meta"`
}

func (r RegisterValue) Json() string {
	str, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}

	return string(str)
}

type RegisterOptions struct {
	// Namespace, multiple spatial data isolation
	Namespace string

	// Service type, optional API or RPC
	ServiceType string

	Key string

	IP string

	Port string

	EtcdConfig EtcdConfig

	Logger *zap.Logger

	// The maximum connection timeout waiting time cannot be less than 10 seconds
	MaxTimeoutRetryTime time.Duration

	// Lease heartbeat time, default 10s
	TimeToLive int64

	// Metadata information, custom fields
	Meta H
}

type RegisterService struct {
	opt RegisterOptions

	fullKey string

	client *clientv3.Client

	leaseId clientv3.LeaseID

	keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse

	quitCh chan struct{}

	closeCh chan struct{}
}

func NewRegisterService(opt RegisterOptions) (*RegisterService, error) {
	if opt.Namespace == "" {
		opt.Namespace = "default"
	}

	if opt.Key == "" {
		panic("service name (Key) cannot be empty")
	}

	if opt.IP == "" || opt.Port == "" {
		panic("IP or Port cannot be empty")
	}

	if opt.IP == "*" {
		var err error
		opt.IP, err = GetOutBoundIP()
		if err != nil {
			opt.IP = "127.0.0.1"
		}
	}

	if len(opt.EtcdConfig.Endpoints) == 0 {
		panic("The connection endpoint is empty")
	}

	if opt.ServiceType != "api" && opt.ServiceType != "rpc" {
		opt.ServiceType = "rpc"
	}

	opt.ServiceType = strings.ToLower(opt.ServiceType)

	if opt.TimeToLive < 1 {
		opt.TimeToLive = 10
	}

	if opt.MaxTimeoutRetryTime > 0 && opt.MaxTimeoutRetryTime < time.Second*10 {
		opt.MaxTimeoutRetryTime = time.Second * 10
	}

	rg := &RegisterService{opt: opt, closeCh: make(chan struct{}, 1)}

	return rg, rg.Register()
}

func (r *RegisterService) Register() (err error) {
	r.client, err = clientv3.New(clientv3.Config(r.opt.EtcdConfig))
	if err != nil {
		r.loggerErr("Failed to create etcd client", err)
		return err
	}

	if err = r.register(); err != nil {
		r.loggerErr("Service registration failed", err)
		return err
	}

	r.quitCh = make(chan struct{}, 1)

	go r.keepAliveAsync()

	return nil
}

func (r *RegisterService) register() error {
	resp, err := r.client.Grant(r.client.Ctx(), r.opt.TimeToLive)
	if err != nil {
		r.loggerErr("Lease creation failed", err)
		return err
	}

	r.leaseId = resp.ID

	// "<Namespace>/services/<ServiceType>/<Key>/<LeaseId>"
	r.fullKey = path.Join(r.opt.Namespace, "services", r.opt.ServiceType, r.opt.Key, strconv.FormatInt(int64(r.leaseId), 10))

	value := RegisterValue{
		Timestamp: time.Now().Unix(),
		IP:        r.opt.IP,
		Port:      r.opt.Port,
		Meta:      r.opt.Meta,
	}

	_, err = r.client.Put(r.client.Ctx(), r.fullKey, value.Json(), clientv3.WithLease(r.leaseId))
	if err != nil {
		r.loggerErr("Put operation failed", err)
		return err
	}

	r.keepAliveCh, err = r.client.KeepAlive(r.client.Ctx(), r.leaseId)
	if err != nil {
		r.loggerErr("KeepAlive operation failed", err)
		return err
	}

	return nil
}

func (r *RegisterService) keepAliveAsync() {
	for {
		select {
		case <-r.closeCh:
			r.loggerInfo("Service health check has stopped")
			return
		case res := <-r.keepAliveCh:
			if res == nil {
				if r.isActive() {
					r.loggerInfo("The service may have stopped abnormally and will be re registered for you soon")
					if err := r.register(); err != nil {
						r.loggerWar("Attempt to re register service failed")
					}
				} else {
					quit, ok := r.retry()
					if !ok {
						r.loggerErr("Attempt to reconnect to server failed", errors.New("nil"))
					}
					if quit {
						r.loggerErr(fmt.Sprintf("The offline reconnection mechanism of the service has reached the maximum waiting time (%v), and the service is about to exit", r.opt.MaxTimeoutRetryTime), errors.New("context expiration"))
						r.quitCh <- struct{}{}
						close(r.quitCh)
						return
					}
				}
			}
		}
	}
}

func (r *RegisterService) retry() (bool, bool) {
	r.loggerWar("Service unavailable, attempting to reconnect")
	ctx := context.Background()
	if r.opt.MaxTimeoutRetryTime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.opt.MaxTimeoutRetryTime)
		defer cancel()
	}

	for _, endpoint := range r.opt.EtcdConfig.Endpoints {
		_, err := r.client.Status(ctx, endpoint)
		if err == nil {
			r.loggerWar("Service reconnection successful!")
			return false, true
		}

		if errors.Is(err, context.DeadlineExceeded) {
			return true, false
		}
	}

	return false, false
}

func (r *RegisterService) isActive() bool {
	r.loggerInfo("Checking if the service is active")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	for _, endpoint := range r.opt.EtcdConfig.Endpoints {
		_, err := r.client.Status(ctx, endpoint)
		if err == nil {
			r.loggerInfo("Active service")
			return true
		}
	}

	return false
}

func (r *RegisterService) Stop() {
	r.closeCh <- struct{}{}
	close(r.closeCh)
	r.unregister()

	if r.client != nil {
		if err := r.client.Close(); err != nil {
			r.loggerErr("etcd client failed to close", err)
		}
	}
}

func (r *RegisterService) ListenQuit() <-chan struct{} {
	return r.quitCh
}

func (r *RegisterService) unregister() {
	ctx, cancel := context.WithTimeout(r.client.Ctx(), time.Second*10)
	defer cancel()

	// Lease cancellation
	if _, err := r.client.Revoke(ctx, r.leaseId); err != nil {
		r.loggerErr("unregister failed", err)
	}

	// Delete key
	if _, err := r.client.Delete(ctx, r.fullKey); err != nil {
		r.loggerErr("unregister failed", err)
	}

	r.loggerInfo("Service has been unregister")
}

func (r *RegisterService) loggerErr(msg string, err error) {
	if r.opt.Logger == nil {
		fmt.Printf("[RegisterService Error]: %s err:%s\n", msg, err.Error())
		return
	}

	r.opt.Logger.Error("[RegisterService]: "+msg, zap.Error(err))
}

func (r *RegisterService) loggerWar(msg string) {
	if r.opt.Logger == nil {
		fmt.Printf("[RegisterService Warning]: %s \n", msg)
		return
	}

	r.opt.Logger.Warn("[RegisterService]: " + msg)
}

func (r *RegisterService) loggerInfo(msg string) {
	if r.opt.Logger == nil {
		fmt.Printf("[RegisterService Warning]: %s \n", msg)
		return
	}

	r.opt.Logger.Info("[RegisterService]: " + msg)
}
