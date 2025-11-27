package main

import (
	"net"

	"github.com/oauth2-proxy/mockoidc"
)

func main() {
	m, err := mockoidc.NewServer(nil)
	if err != nil {
		panic(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		panic(err)
	}
	err = m.Start(ln, nil)
	if err != nil {
		panic(err)
	}

	m.Config()

}
