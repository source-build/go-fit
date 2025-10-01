package frpc

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"sync"

	"github.com/source-build/go-fit"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
)

// ServiceInstance 表示一个服务实例，包含etcd key和gRPC地址信息
type ServiceInstance struct {
	Key     string           // etcd中的完整key，用于删除时比对
	Address resolver.Address // gRPC地址信息
}

const EtcdScheme = "etcd"

var registerValuePool = sync.Pool{
	New: func() interface{} {
		return &fit.RegisterValue{}
	},
}

var (
	NoAvailableServiceErr = status.Error(codes.Unavailable, "no available services")
	UpdateGRPCStateErr    = status.Error(codes.Internal, "update grpc state failed")
)

// 注册etcd解析器
func registerEtcdResolver(conf RpcClientConf) {
	resolver.Register(&etcdBuilder{conf.EtcdClient})
}

// Monitor updates for specified targets, including address updates and service configuration updates
type etcdResolver struct {
	mu           sync.RWMutex
	client       *clientv3.Client
	cc           resolver.ClientConn
	fullKey      string
	addressCache []ServiceInstance
	ctx          context.Context
	cancel       context.CancelFunc
}

func (r *etcdResolver) ResolveNow(resolver.ResolveNowOptions) {
}

func (r *etcdResolver) Close() {
	r.cancel()
}

func (r *etcdResolver) getAddress() error {
	if r.addressCache == nil {
		r.addressCache = make([]ServiceInstance, 0)
	}

	r.ctx, r.cancel = context.WithCancel(r.client.Ctx())
	response, err := r.client.Get(r.client.Ctx(), r.fullKey, clientv3.WithPrefix())
	if err != nil {
		r.cc.ReportError(err)
		return err
	}

	if len(response.Kvs) == 0 {
		r.cc.ReportError(NoAvailableServiceErr)
		return NoAvailableServiceErr
	}

	for _, kv := range response.Kvs {
		if instance, ok := r.processKvPair(kv); ok {
			r.addressCache = append(r.addressCache, instance)
		}
	}

	if len(r.addressCache) == 0 {
		r.cc.ReportError(NoAvailableServiceErr)
		return NoAvailableServiceErr
	}

	err = r.cc.UpdateState(resolver.State{Addresses: r.extractAddresses()})
	if err != nil {
		r.cc.ReportError(err)
		return UpdateGRPCStateErr
	}

	return nil
}

// 处理单个etcd键值对，转换为ServiceInstance
func (r *etcdResolver) processKvPair(kv *mvccpb.KeyValue) (ServiceInstance, bool) {
	rg := registerValuePool.Get().(*fit.RegisterValue)
	defer registerValuePool.Put(rg)
	if err := json.Unmarshal(kv.Value, rg); err != nil {
		log.Printf("[etcdResolver] Failed to unmarshal service data: %v", err)
		return ServiceInstance{}, false
	}

	addr := resolver.Address{Addr: net.JoinHostPort(rg.IP, rg.Port)}

	if weight := rg.Meta.Value("weight"); weight != nil {
		addr.Attributes = attributes.New("weight", weight)
	}

	instance := ServiceInstance{
		Key:     string(kv.Key),
		Address: addr,
	}

	return instance, true
}

// 提取Address列表传递给gRPC
func (r *etcdResolver) extractAddresses() []resolver.Address {
	addresses := make([]resolver.Address, len(r.addressCache))
	for i, instance := range r.addressCache {
		addresses[i] = instance.Address
	}
	return addresses
}

func (r *etcdResolver) watcher() {
	rch := r.client.Watch(r.client.Ctx(), r.fullKey, clientv3.WithPrefix())
	for {
		select {
		case <-r.ctx.Done():
			return
		case v := <-rch:
			r.handlerEvents(v)
		}
	}
}

func (r *etcdResolver) handlerEvents(resp clientv3.WatchResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ev := range resp.Events {
		switch ev.Type {
		case clientv3.EventTypePut:
			if instance, ok := r.processKvPair(ev.Kv); ok {
				r.addOrUpdateInstance(instance)
			}
		case clientv3.EventTypeDelete:
			r.removeInstanceByKey(string(ev.Kv.Key))
		}
	}

	addresses := r.extractAddresses()
	if len(addresses) == 0 {
		r.cc.ReportError(NoAvailableServiceErr)
		return
	}

	err := r.cc.UpdateState(resolver.State{Addresses: addresses})
	if err != nil {
		r.cc.ReportError(err)
		return
	}
}

// 添加或更新服务实例
func (r *etcdResolver) addOrUpdateInstance(newInstance ServiceInstance) {
	for i, instance := range r.addressCache {
		if instance.Key == newInstance.Key {
			r.addressCache[i] = newInstance
			return
		}
	}
	r.addressCache = append(r.addressCache, newInstance)
}

// 通过key删除服务实例
func (r *etcdResolver) removeInstanceByKey(key string) {
	for i, instance := range r.addressCache {
		if instance.Key == key {
			r.addressCache = append(r.addressCache[:i], r.addressCache[i+1:]...)
			return
		}
	}
}
