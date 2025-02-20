package fit

import (
	"errors"
	"go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"time"
)

var EtcdV3Client *clientv3.Client

func NewEtcd(config clientv3.Config, notReconnect ...bool) error {
	if EtcdV3Client != nil {
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

	EtcdV3Client = clientV3

	return nil
}

func CloseEtcd() error {
	if EtcdV3Client == nil {
		return nil
	}

	return EtcdV3Client.Close()
}
