package fit

import (
	"context"
	"encoding/json"
	"errors"
	"go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"time"
)

var client *clientv3.Client

type EtcdHandle struct {
	EtcdClient   *clientv3.Client
	leaseID      clientv3.LeaseID
	ctx          context.Context
	KeepRespChan <-chan *clientv3.LeaseKeepAliveResponse
}

func InitEtcd(config clientv3.Config, notReconnect ...bool) error {
	if client != nil {
		return errors.New("instance already exists")
	}
	if len(notReconnect) == 0 {
		if config.DialTimeout == 0 {
			config.DialTimeout = time.Second * 30
		}
		config.DialOptions = []grpc.DialOption{
			grpc.WithBlock(),
		}
	}
	clientV3, err := clientv3.New(config)
	if err != nil {
		return err
	}
	client = clientV3
	return nil
}

func MainEtcdv3(ctx ...context.Context) (*EtcdHandle, error) {
	if client == nil {
		return nil, NewErr("etcd instance not found!")
	}
	ctx1 := context.Background()
	if len(ctx) > 0 {
		ctx1 = ctx[0]
	}
	return &EtcdHandle{
		EtcdClient: client,
		ctx:        ctx1,
	}, nil
}

func MainEtcdClientv3() *clientv3.Client {
	return client
}

func CloseEtcd() {
	if err := client.Close(); err != nil {
		Error(err)
	}
}

// Get key
func (e *EtcdHandle) Get(key string, ops ...[]clientv3.OpOption) (*clientv3.GetResponse, error) {
	var opss []clientv3.OpOption
	if len(ops) > 0 {
		opss = ops[0]
	}
	getResp, err := e.EtcdClient.Get(e.ctx, key, opss...)
	if err != nil {
		return nil, err
	}
	return getResp, nil
}

func (e *EtcdHandle) Put(key, val string, leases ...int64) (*clientv3.PutResponse, error) {
	if len(leases) > 0 {
		leaseResp, err := e.EtcdClient.Grant(e.ctx, leases[0])
		if err != nil {
			return nil, err
		}

		e.leaseID = leaseResp.ID
		putResp, err := e.EtcdClient.Put(e.ctx, key, val, clientv3.WithLease(e.leaseID))
		if err != nil {
			return nil, err
		}
		return putResp, nil
	}

	getResp, err := e.EtcdClient.Put(e.ctx, key, val)
	if err != nil {
		return nil, err
	}
	return getResp, nil
}

func (e *EtcdHandle) KeepAlive() error {
	if e.leaseID == 0 {
		return errors.New("leaseID does not exist")
	}
	keepRespChan, err := e.EtcdClient.KeepAlive(e.ctx, e.leaseID)
	if err != nil {
		return err
	}

	e.KeepRespChan = keepRespChan
	go func(e *EtcdHandle) {
		for {
			select {
			case _, ok := <-e.KeepRespChan:
				if !ok {
					return
				}
			case <-e.ctx.Done():
				return
			}
		}
	}(e)
	return nil
}

func (e *EtcdHandle) Revoke() error {
	if e.leaseID > 0 {
		_, err := e.EtcdClient.Revoke(e.ctx, e.leaseID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *EtcdHandle) GetByPrefix(prefix string) (*clientv3.GetResponse, error) {
	getResp, err := e.EtcdClient.Get(e.ctx, prefix, []clientv3.OpOption{clientv3.WithPrefix()}...)
	if err != nil {
		return nil, err
	}
	return getResp, nil
}

func (e *EtcdHandle) WatchPrefix() func(prefix string, data chan *clientv3.Event) {
	return func(prefix string, data chan *clientv3.Event) {
		watcher := e.EtcdClient.Watch(e.ctx, prefix, clientv3.WithPrefix())
		for {
			select {
			case res := <-watcher:
				for _, event := range res.Events {
					data <- event
				}
			case <-e.ctx.Done():
				close(data)
				return
			}
		}
	}
}

// ExtractKVUtil extract key and value
func ExtractKVUtil(resp *clientv3.GetResponse) (key []byte, val []byte) {
	for _, kv := range resp.Kvs {
		return kv.Key, kv.Value
	}
	return
}

// ExtractKeyUtil extract key
func ExtractKeyUtil(resp *clientv3.GetResponse) string {
	var key string
	for _, kv := range resp.Kvs {
		key = string(kv.Key)
		break
	}
	return key
}

// ExtractValUtil extract value
func ExtractValUtil(resp *clientv3.GetResponse) string {
	var value string
	for _, kv := range resp.Kvs {
		value = string(kv.Value)
		break
	}
	return value
}

// ExtractWatchChanTypeUtil extract watch chan value type
func ExtractWatchChanTypeUtil(resp clientv3.WatchResponse) string {
	var value string
	for _, kv := range resp.Events {
		value = kv.Type.String()
		break
	}
	return value
}

// ExtractWatchChanValUtil extract watch chan value
func ExtractWatchChanValUtil(resp clientv3.WatchResponse) []byte {
	var value []byte
	for _, kv := range resp.Events {
		value = kv.Kv.Value
		break
	}
	return value
}

func ExtractValAndToMap(resp *clientv3.GetResponse) (result map[string]any, err error) {
	var value string
	result = make(map[string]any)
	for _, kv := range resp.Kvs {
		value = string(kv.Value)
		break
	}
	err = json.Unmarshal([]byte(value), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func PingEtcd(ctx ...context.Context) error {
	var rootCtx context.Context
	if len(ctx) > 0 {
		rootCtx = ctx[0]
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		rootCtx = ctx
	}

	_, err := MainEtcdClientv3().Get(rootCtx, "/")
	if err != nil {
		return err
	}
	return nil
}
