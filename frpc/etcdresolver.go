package frpc

import (
	"context"
	"encoding/json"
	"github.com/source-build/go-fit"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"sync"
)

const EtcdScheme = "etcd"

var (
	NoAvailableServiceErr = status.Error(codes.Unavailable, "no available services")
)

func registerResolver(conf RpcClientConf) {
	resolver.Register(&etcdBuilder{conf.EtcdClient})
}

// Monitor updates for specified targets, including address updates and service configuration updates
type etcdResolver struct {
	mu           sync.RWMutex
	client       *clientv3.Client
	cc           resolver.ClientConn
	fullKey      string
	addressCache []resolver.Address
	ctx          context.Context
	cancel       context.CancelFunc
}

func (r *etcdResolver) ResolveNow(resolver.ResolveNowOptions) {
}

func (r *etcdResolver) Close() {
	r.cancel()
}

func (r *etcdResolver) newAddress() {
	r.ctx, r.cancel = context.WithCancel(r.client.Ctx())

	response, err := r.client.Get(r.client.Ctx(), r.fullKey, clientv3.WithPrefix())
	if err != nil {
		r.cc.ReportError(err)
		return
	}

	if len(response.Kvs) == 0 {
		r.cc.ReportError(NoAvailableServiceErr)

		return
	}

	addresses := make([]resolver.Address, 0)
	for _, kv := range response.Kvs {
		var rg fit.RegisterValue
		if err = json.Unmarshal(kv.Value, &rg); err != nil {
			log.Println("[etcdResolver]: ", err)
			continue
		}

		addr := resolver.Address{ServerName: string(kv.Key), Addr: net.JoinHostPort(rg.IP, rg.Port)}

		weight := rg.Meta.Value("weight")
		if weight != nil {
			addr.Attributes = attributes.New("weight", weight)
		}

		addresses = append(addresses, addr)
	}

	if len(addresses) == 0 {
		r.cc.ReportError(NoAvailableServiceErr)
		return
	}

	r.addressCache = addresses

	err = r.cc.UpdateState(resolver.State{
		Addresses: addresses,
	})
	if err != nil {
		r.cc.ReportError(err)
		return
	}
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

func (r *etcdResolver) handlerEvents(wresp clientv3.WatchResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ev := range wresp.Events {
		switch ev.Type {
		// Add or modify
		case clientv3.EventTypePut:
			var rg fit.RegisterValue
			if err := json.Unmarshal(ev.Kv.Value, &rg); err != nil {
				log.Println("[etcdResolver watcher Error]: ", err)
				continue
			}

			addr := resolver.Address{ServerName: string(ev.Kv.Key), Addr: net.JoinHostPort(rg.IP, rg.Port)}
			if !r.addressExists(addr) {
				r.addressCache = append(r.addressCache, addr)
			}
		// Delete
		case clientv3.EventTypeDelete:
			r.removeAddress(string(ev.Kv.Key))
		}
	}

	err := r.cc.UpdateState(resolver.State{
		Addresses: r.addressCache,
	})
	if err != nil {
		r.cc.ReportError(err)
		return
	}
}

// Check if the address already exists
func (r *etcdResolver) addressExists(addr resolver.Address) bool {
	for _, a := range r.addressCache {
		if a.ServerName == addr.ServerName {
			return true
		}
	}

	return false
}

// Remove address from cache
func (r *etcdResolver) removeAddress(serverName string) {
	for i, addr := range r.addressCache {
		if addr.ServerName == serverName {
			r.addressCache = append(r.addressCache[:i], r.addressCache[i+1:]...)
			return
		}
	}
}
