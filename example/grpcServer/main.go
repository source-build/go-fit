package main

func main() {
	////连接etcd
	//client, err := clientv3.New(clientv3.Config{
	//	Endpoints:   []string{"127.0.0.1:2479"},
	//	DialTimeout: time.Second * 5,
	//})
	//if err != nil {
	//	log.Fatalln("连接失败", err)
	//}
	//
	////create tls
	//cred, err := fit.NewServiceTLS(&fit.CertPool{
	//	CertFile: "keys/server.crt",
	//	KeyFile:  "keys/server.key",
	//	CaCert:   "keys/ca.crt",
	//})
	//if err != nil {
	//	fit.Fatal("create tls failed err:" + err.Error())
	//}
	//
	//listen, err := net.Listen("tcp", ":8000")
	//if err != nil {
	//	log.Fatalln(err)
	//}
	//
	//rpcServer := grpc.NewServer(grpc.Creds(cred))
	//
	//pb.RegisterEmailPinServer(rpcServer, new(User))
	//
	////进行映射绑定。在指定的gRPC服务器上注册服务器反射服务。
	////reflection.Register(rpcServer)
	//
	//var s *fit.ServiceRegister
	//quit := make(chan os.Signal, 1)
	//go func() {
	//	signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGKILL)
	//	localIp, _ := fit.GetOutBoundIP()
	//	s, err = fit.NewServiceRegister(&fit.ServiceRegister{
	//		Ctx:    context.Background(),
	//		Client: client,
	//		Key:    "/serves/rpc/dpp/Mjhd",
	//		Value:  localIp + ":8000",
	//		Lease:  10,
	//	})
	//	if err != nil {
	//		log.Fatalln(err)
	//	}
	//	fmt.Println("service start success!!!")
	//	if err := rpcServer.Serve(listen); err != nil {
	//		log.Fatalln(err)
	//	}
	//}()
	//<-quit
	//s.Close()
	//fmt.Println("service close!")
}
