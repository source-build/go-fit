package frpc

import (
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
	"path"
)

// Builder 用于监视名称解析更新
type etcdBuilder struct {
	Client *clientv3.Client
}

func (b *etcdBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	r := &etcdResolver{
		client:  b.Client,
		cc:      cc,
		fullKey: path.Join(rpcClientConf.GetNamespace(), "rpc", target.URL.Host),
	}

	r.newAddress()

	go r.watcher()

	return r, nil
}

func (b *etcdBuilder) Scheme() string {
	return EtcdScheme
}
