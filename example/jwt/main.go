package main

import (
	"fmt"
	"github.com/source-build/go-fit"
	"log"
	"time"
)

func main() {
	key := "lpl654"
	//生成token
	jwtClaims := fit.JwtClaims{
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
		Id:        "45565",
		Subject:   "user_login",
	}
	str, err := fit.NewJwtClaims(key, jwtClaims)
	if err != nil {
		log.Fatalln(str)
	}
	fmt.Println(str)

	//验证token
	t, err := fit.Valid(key, str)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("success")
	fmt.Printf("%+v", t)
}
