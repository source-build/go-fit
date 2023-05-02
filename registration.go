package fit

import (
	"context"
	"go.etcd.io/etcd/client/v3"
	"os"
	"time"
)

type ServiceRegister struct {
	Ctx           context.Context
	Client        *clientv3.Client
	Key           string
	Value         string
	Lease         int64
	leaseID       clientv3.LeaseID
	keepAliveChan <-chan *clientv3.LeaseKeepAliveResponse

	//Triggered when ETCD KeepAlive fails
	OnBack func()

	//Pass a listening chan. When KeepAlive fails, it will send a SignalTag signal to this chan
	SignalChan chan os.Signal

	//The signal sent when closing is os.Kill by default
	SignalTag os.Signal
}

func NewServiceRegister(config *ServiceRegister) (*ServiceRegister, error) {
	if err := config.putKeyWithLease(config.Lease); err != nil {
		return nil, err
	}
	return config, nil
}

func (e *ServiceRegister) putKeyWithLease(lease int64) error {
	// create lease
	grant, err := e.Client.Grant(e.Ctx, lease)
	if err != nil {
		return err
	}

	// put
	_, err = e.Client.Put(e.Ctx, e.Key, e.Value, clientv3.WithLease(grant.ID))
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
	go e.keepAlive()
	return nil
}

// Close cancellation of lease
func (e *ServiceRegister) Close() {
	ctx, cancel := context.WithTimeout(e.Ctx, time.Second*10)
	defer cancel()
	if _, err := e.Client.Revoke(ctx, e.leaseID); err != nil {
		Error("[ETCD Revoke]: err:" + err.Error())
	}
	if err := e.Client.Close(); err != nil {
		Error("[ETCD Close]: err:" + err.Error())
	}
}

func (e *ServiceRegister) keepAlive() {
	for {
		select {
		case resp := <-e.keepAliveChan:
			if resp == nil {
				e.exit()
				return
			}
		}
	}
}

func (e *ServiceRegister) exit() {
	if e.OnBack != nil {
		e.OnBack()
	}
	if e.SignalChan != nil {
		if e.SignalTag == nil {
			e.SignalTag = os.Kill
		}
		e.SignalChan <- e.SignalTag
	}
}
