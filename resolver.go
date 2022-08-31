package fit

import (
	"context"
	"go.etcd.io/etcd/client/v3"
	"sync"

	"google.golang.org/grpc/resolver"
)

type Resolver struct {
	sync.RWMutex
	Client    *clientv3.Client
	cc        resolver.ClientConn
	prefix    string
	addresses map[string]resolver.Address
}

func (r *Resolver) ResolveNow(resolver.ResolveNowOptions) {
	// TODO:watch It will be called after changes
}

func (r *Resolver) Close() {
	// TODO: Parser off
}

func (r *Resolver) watcher() {
	r.addresses = make(map[string]resolver.Address)

	response, err := r.Client.Get(context.Background(), r.prefix, clientv3.WithPrefix())
	if err != nil {
		return
	}
	for _, kv := range response.Kvs {
		r.setAddress(string(kv.Key), string(kv.Value))
	}
	r.cc.UpdateState(resolver.State{
		Addresses: r.getAddresses(),
	})
	//TODO: Listening has been canceled here
}

func (r *Resolver) setAddress(key, value string) {
	r.Lock()
	defer r.Unlock()
	r.addresses[key] = resolver.Address{Addr: value}
}

func (r *Resolver) delAddress(key string) {
	r.Lock()
	defer r.Unlock()
	delete(r.addresses, key)
}

func (r *Resolver) getAddresses() []resolver.Address {
	addresses := make([]resolver.Address, 0, len(r.addresses))
	for _, address := range r.addresses {
		addresses = append(addresses, address)
	}
	return addresses
}
