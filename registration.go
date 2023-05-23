package fit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"net/http"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"
)

var serviceRegisterInstance *ServiceRegister

type ServiceRegister struct {
	Ctx           context.Context
	Client        *clientv3.Client
	Key           string
	Value         string
	Lease         int64
	leaseID       clientv3.LeaseID
	keepAliveChan <-chan *clientv3.LeaseKeepAliveResponse
	restartChan   chan struct{}

	isCallClose bool
	runRetry    bool

	// Number of unexpected exits (disconnection and reconnection), 0 no retry(default).
	RetryCount        int
	RetryWaitDuration time.Duration
	RetryFunc         func(count int)
	RetryOkFunc       func()
	RetryWaitMultiple bool

	// Whether to use environment isolation is only effective for the development environment,
	// as it uses the same registry. When collaborating on development, multiple services may be located on different LANs, resulting in service discovery using a non local network.
	// It is effective when 'env' is' development '
	UseIsolate bool

	// If it is a development environment and UseIsolate is true, then this service will be bound to the machine where it is located.
	Env EnvType

	NoSuffix bool

	MID string

	cancel context.CancelFunc

	//Triggered when ETCD KeepAlive fails
	OnBack func()

	//When the Status field changes, this function will be called.
	//You can determine this value in the middleware.
	//When the value is equal to ServiceStatusRun,
	//it indicates that the service is back to normal and can continue to provide services.
	//When the value is equal to ServiceStatusWaitDone,
	//it indicates that the service will no longer receive new requests and is currently waiting for the requests being processed to complete.
	//After all the request tasks in the service are completed, you should close this service,
	//The registration center will no longer forward traffic for this service until it is shut down and restarted
	OnStatusChange func(value RegisterCenterValue, config *ServiceRegister)

	//Pass a listening chan. When KeepAlive fails, it will send a SignalTag signal to this chan
	SignalChan chan os.Signal

	//The signal sent when closing is os.Kill by default
	SignalTag os.Signal
}

func GetLocalMid() string {
	if serviceRegisterInstance == nil {
		return ""
	}
	if serviceRegisterInstance.Env != EnvDevelopment {
		return ""
	}
	return serviceRegisterInstance.MID
}

func NewServiceRegister(config *ServiceRegister) (*ServiceRegister, error) {
	split := strings.Split(config.Key, "/")
	if !config.NoSuffix {
		split = append(split, NewRandom().Char(6))
		config.Key = "/" + path.Join(split...)
	}

	if config.UseIsolate && config.Env == EnvDevelopment {
		if config.MID == "" {
			config.MID = MD5encryption(GetMachineCode())
		}
		final := split[len(split)-1]
		split = split[:len(split)-1]
		split = append(split, config.MID)
		split = append(split, final)
		config.Key = "/" + path.Join(split...)
	}

	config.Ctx, config.cancel = context.WithCancel(config.Ctx)
	if err := config.putKeyWithLease(config.Lease); err != nil {
		return nil, err
	}

	serviceRegisterInstance = config
	return config, nil
}

func (e *ServiceRegister) putKeyWithLease(lease int64, newVal ...string) error {
	// create lease
	grant, err := e.Client.Grant(e.Ctx, lease)
	if err != nil {
		return err
	}

	// put
	var value string
	if len(newVal) > 0 && newVal[0] != "" {
		value = newVal[0]
	} else {
		value = e.Value
	}

	_, err = e.Client.Put(e.Ctx, e.Key, value, clientv3.WithLease(grant.ID))
	if err != nil {
		return err
	}

	// keep lease
	leaseRespChan, err := e.Client.KeepAlive(e.Ctx, grant.ID)
	if err != nil {
		return err
	}

	e.leaseID = grant.ID
	e.keepAliveChan = leaseRespChan
	e.restartChan = make(chan struct{}, 1)
	go e.watcher()
	go e.keepAlive()
	return nil
}

// Close cancellation of lease
func (e *ServiceRegister) Close() {
	e.isCallClose = true
	ctx, cancel := context.WithTimeout(e.Ctx, time.Second*10)
	defer cancel()
	if _, err := e.Client.Revoke(ctx, e.leaseID); err != nil {
		Error("[ETCD Revoke]: err:" + err.Error())
	}
}

func (e *ServiceRegister) watcher() {
	watchChan := e.Client.Watch(e.Ctx, e.Key)
	for watchResponse := range watchChan {
		for _, event := range watchResponse.Events {
			if event.Type == clientv3.EventTypeDelete {
				if !e.isCallClose {
					e.restartChan <- struct{}{}
					if _, err := e.Client.Revoke(e.Ctx, e.leaseID); err != nil {
						Error("msg", "ETCD Revoke failed!", "err", err)
					}
					if err := e.putKeyWithLease(e.Lease, string(event.Kv.Value)); err != nil {
						Error("msg", "service restart failed!", "err", err)
						e.Client.Delete(e.Ctx, e.Key)
						e.cancel()
						e.exit()
						if e.OnBack != nil {
							e.OnBack()
						}
					}
				}
				return
			}
			var rcv RegisterCenterValue
			if err := json.Unmarshal(event.Kv.Value, &rcv); err != nil {
				// invalid data
				e.Client.Delete(e.Ctx, e.Key)
				e.cancel()
				return
			}

			if rcv.Status == ServiceStatusKill {
				Info("Received kill instruction")
				e.Client.Delete(e.Ctx, e.Key)
				e.cancel()
				return
			}

			if e.OnStatusChange != nil {
				go e.OnStatusChange(rcv, e)
			}

			// update
			if event.Type == clientv3.EventTypePut {
				e.restartChan <- struct{}{}
				if _, err := e.Client.Revoke(e.Ctx, e.leaseID); err != nil {
					Error("msg", "ETCD Revoke failed!", "err", err)
				}
				if err := e.putKeyWithLease(e.Lease, string(event.Kv.Value)); err != nil {
					Error("msg", "service restart failed!", "err", err)
					e.Client.Delete(e.Ctx, e.Key)
					e.cancel()
					e.exit()
					if e.OnBack != nil {
						e.OnBack()
					}
				}
				return
			}
			break
		}
	}
}

func (e *ServiceRegister) keepAlive() {
	var isRestart bool
	var ctxDone bool
	defer func() {
		if isRestart {
			return
		}
		if ctxDone {
			return
		}
		if e.RetryCount == 0 {
			e.exit()
			if e.OnBack != nil {
				e.OnBack()
			}
			return
		}
		go e.retry()
	}()
	for {
		select {
		case <-e.restartChan:
			isRestart = true
			return
		case <-e.Ctx.Done():
			ctxDone = true
			return
		case resp := <-e.keepAliveChan:
			if resp == nil {
				return
			}
		}
	}
}

func (e *ServiceRegister) retry() {
	if e.runRetry || e.RetryCount < 1 {
		return
	}
	defer func() {
		e.runRetry = false
	}()
	e.runRetry = true

	if e.RetryWaitDuration == 0 {
		e.RetryWaitDuration = time.Second * 5
	}
	rd := e.RetryWaitDuration
	t := time.NewTicker(rd)
	defer t.Stop()
	retryCount := 0
	for {
		select {
		case <-e.Ctx.Done():
			e.exit()
			return
		case <-t.C:
			retryCount++
			if e.RetryFunc != nil {
				e.RetryFunc(e.RetryCount)
			}
			if e.RetryWaitMultiple {
				t.Stop()
				t = time.NewTicker(rd)
				rd *= 2
			}

			ctx, cancel := context.WithTimeout(e.Ctx, time.Second*5)
			if _, err := e.Client.MemberList(ctx); err == nil {
				cancel()
				if e.RetryOkFunc != nil {
					e.RetryOkFunc()
				}
				return
			}
			cancel()
			Error(fmt.Sprintf("[Service offline]: Unable to connect to the registry, attempting to reconnect, retried %d times.", retryCount))
			if retryCount >= e.RetryCount {
				e.exit()
				return
			}
		}
	}
}

func (e *ServiceRegister) exit() {
	if e.restartChan != nil {
		close(e.restartChan)
	}
	if e.SignalChan != nil {
		if e.SignalTag == nil {
			e.SignalTag = os.Interrupt
		}
		e.SignalChan <- e.SignalTag
	}
}

func (e *ServiceRegister) Shutdown() {
	e.exit()
}

func (e *ServiceRegister) Restore(value RegisterCenterValue) error {
	value.Status = ServiceStatusRun
	result, err := json.Marshal(&value)
	if err != nil {
		return err
	}
	if _, err := e.Client.Put(e.Ctx, e.Key, string(result)); err != nil {
		return err
	}
	return nil
}

type StatUnfinished struct {
	data         int32
	waitDone     bool
	NotAvailable bool
	Signal       chan struct{}
}

func NewStatUnfinished(option ...*StatUnfinished) *StatUnfinished {
	if len(option) > 0 {
		return option[0]
	}
	return &StatUnfinished{}
}

func (s *StatUnfinished) GinStatUnfinished() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.waitDone || s.NotAvailable {
			c.AbortWithStatusJSON(http.StatusBadRequest, ResponseOK{
				Code: StatusCErr,
				Msg:  "故障已转移,请重试",
			})
			return
		}
		s.Add()
		c.Next()
		s.Sub()
	}
}

func (s *StatUnfinished) GrpcStatUnfinished() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		if s.waitDone || s.NotAvailable {
			return nil, errors.New("故障已转移,请重试")
		}
		s.Add()
		res, err := handler(ctx, req)
		s.Sub()
		return res, err
	}
}

func (s *StatUnfinished) GrpcHandleStatUnfinished() error {
	if s.waitDone || s.NotAvailable {
		return errors.New("故障已转移,请重试")
	}
	return nil
}

func (s *StatUnfinished) FiringWaitDone() {
	s.waitDone = true
}

func (s *StatUnfinished) Restore() {
	s.waitDone = false
}

func (s *StatUnfinished) Add() {
	atomic.AddInt32(&s.data, 1)
}

func (s *StatUnfinished) Sub() {
	atomic.AddInt32(&s.data, -1)
	if s.data == 0 && s.waitDone {
		s.Signal <- struct{}{}
	}
}

func (s *StatUnfinished) Value() int32 {
	return s.data
}

func (s *StatUnfinished) SetAvailable(is bool) {
	s.NotAvailable = !is
}
