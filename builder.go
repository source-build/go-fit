package fit

import (
	"fmt"
	"go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
)

type Builder struct {
	Client *clientv3.Client
}

func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	prefix := fmt.Sprintf("/%s/", target.Endpoint)

	r := &Resolver{
		Client: b.Client,
		cc:     cc,
		prefix: prefix,
	}

	go r.watcher()
	return r, nil
}

func (b *Builder) Scheme() string {
	return "etcd"
}
