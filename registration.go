package fit

import (
	"context"
	"go.etcd.io/etcd/client/v3"
)

type ServiceRegister struct {
	Ctx           context.Context
	Client        *clientv3.Client
	Key           string
	Value         string
	Lease         int64
	leaseID       clientv3.LeaseID
	keepAliveChan <-chan *clientv3.LeaseKeepAliveResponse
}

func NewServiceRegister(config *ServiceRegister) (*ServiceRegister, error) {
	if err := config.putKeyWithLease(config.Lease); err != nil {
		return nil, err
	}
	return config, nil
}

// set lease
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
	_, err := e.Client.Revoke(e.Ctx, e.leaseID)
	if err != nil {
		Error("[ETCD Revoke]: err:" + err.Error())
	}
	if err := e.Client.Close(); err != nil {
		Error("[ETCD close]: err:" + err.Error())
	}
}

func (e *ServiceRegister) keepAlive() {
	for {
		select {
		case resp := <-e.keepAliveChan:
			if resp == nil {
				Info("[ETCD Lease]: lease expired")
				return
			}
		}
	}
}
