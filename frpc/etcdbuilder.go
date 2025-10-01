package frpc

import (
	"errors"
	"log"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
)

type etcdBuilder struct {
	Client *clientv3.Client
}

func (b *etcdBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	r := &etcdResolver{
		client:  b.Client,
		cc:      cc,
		fullKey: b.buildFullKey(target),
	}

	err := r.getAddress()
	if err != nil && (!errors.Is(err, NoAvailableServiceErr) && !errors.Is(err, UpdateGRPCStateErr)) {
		log.Printf("[etcdBuilder] Initial service discovery failed, but starting watcher: %v", err)
		return nil, err
	}

	go r.watcher()

	return r, nil
}

func (b *etcdBuilder) Scheme() string {
	return EtcdScheme
}

func (b *etcdBuilder) buildFullKey(target resolver.Target) string {
	var builder strings.Builder
	builder.Grow(len(rpcClientConf.GetNamespace()) + len(target.URL.Host) + 30)

	builder.WriteString("/")
	builder.WriteString(rpcClientConf.GetNamespace())
	builder.WriteString("/services/")
	builder.WriteString("rpc")
	builder.WriteString("/")
	builder.WriteString(target.URL.Host)
	builder.WriteString("/")

	return builder.String()
}
