package main

import (
	"log"

	"github.com/armon/go-socks5"
)

func main() {
	server, err := socks5.New(&socks5.Config{})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(server.ListenAndServe("tcp", "0.0.0.0:1080"))
}
