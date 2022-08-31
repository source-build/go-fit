package main

func main() {
	//client, err := clientv3.New(clientv3.Config{
	//	Endpoints:   []string{"127.0.0.1:2479"},
	//	DialTimeout: time.Second * 5,
	//})
	//
	//err = fit.NewGrpcClientBuilder(fit.GrpcBuilderConfig{
	//	EtcdClient:         client,
	//	ClientCertPath:     "keys/client.crt",
	//	ClientKeyPath:      "keys/client.key",
	//	RootCrtPath:        "keys/ca.crt",
	//	ServerNameOverride: "sourcebuild.cn",
	//})
	//if err != nil {
	//	log.Fatalln(err)
	//}
	//
	////无参数则直接调用grpc.Dial
	//conn, err := fit.GrpcDial("/serves/rpc/dpp11", fit.Attempts(5))
	//if err != nil {
	//	log.Fatalln("连接失败", err)
	//}
	//defer conn.Close()
	//
	//c := pb.NewEmailPinClient(conn)
	//resp, err := c.ProofEmailPin(context.Background(), &pb.Request{Email: "11"})
	//if err != nil {
	//	log.Fatalln("请求失败", err)
	//}
	//
	//fmt.Println(resp.Msg)
}
