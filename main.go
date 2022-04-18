package main

import (
	"flag"
	"fmt"
	"github.com/hktalent/ersp/core"
	"log"
	"os"

	"github.com/hashicorp/yamux"
)

var session *yamux.Session

func main() {
	socks := flag.String("socks", "0:1080", "socks address:port")
	key := flag.String("key", "303DC5F3-1251-4F11-85D2-168BB2325D9F", "key your all node")

	flag.Usage = func() {
		fmt.Println("rsocks - reverse socks5 server/client")
		fmt.Println("https://github.com/brimstone/rsocks")
		fmt.Println("")
		fmt.Println("Usage:")
		fmt.Println("1) Start rsocks -socks 127.0.0.1:1080 on the client.")
		fmt.Println("2) Start rsocks on the server.")
		fmt.Println("3) Connect to 127.0.0.1:1080 on the client with any socks5 client.")
		fmt.Println("4) Enjoy. :]")
	}

	flag.Parse()
	rr := core.NewReverseSocks5(*key)
	if *socks != "" {
		log.Println("Starting to listen for clients")
		go rr.ListenForSocks()
		log.Fatal(rr.ListenForClients(*socks))
	}

	log.Fatal(rr.ConnectForSocks())

	fmt.Fprintf(os.Stderr, "You must specify a listen port or a connect address")
	os.Exit(1)
}
