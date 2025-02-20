package frpc

import (
	"fmt"
	c "github.com/smartystreets/goconvey/convey"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"testing"
)

func TestNewClient(t *testing.T) {
	clientV3, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	})
	if err != nil {
		t.Errorf("clientv3.New() error = %v", err)
		return
	}

	// 调用此方法进行初始化以完成注册
	err = Init(RpcClientConf{
		// etcd
		EtcdClient: clientV3,
		// 命名空间
		Namespace: "ht",
	})
	if err != nil {
		t.Errorf("Init() error = %v", err)
		return
	}

	type args struct {
		target string
		opts   []DialOption
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: EtcdScheme, args: args{target: "user", opts: []DialOption{WithGrpcOption(grpc.WithTransportCredentials(insecure.NewCredentials()))}}, wantErr: false},
	}

	for index, tt := range tests {
		c.Convey(fmt.Sprintf("Pal%d", index), t, func() {
			_, err := NewClient(tt.args.target, tt.args.opts...)
			c.So(err, c.ShouldEqual, nil)
		})
	}
}
