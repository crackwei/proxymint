package main

import (
	"log"
	"net"

	"net/http"
	_ "net/http/pprof"

	"github.com/BTCChina/mining-pool-proxy/server"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags | log.Lmicroseconds)

	cfg, err := server.LoadConfig()
	if err != nil {
		log.Fatalln("Could not load configuration:", err)
	}

	server, err := server.NewProxy(cfg)
	if err != nil {
		log.Fatalln("Could not start proxy server", err)
	}

	// Enable profiling
	go func() {
		log.Println("Listening on: http://"+cfg.PProfHost, "(pprof)")
		log.Println(http.ListenAndServe(cfg.PProfHost, nil))
	}()

	// Set up the tcp server for stratum
	addr, err := net.ResolveTCPAddr("tcp", cfg.Host)
	if err != nil {
		log.Fatalln("Could not resolve address:", err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatalln("Could not create listener:", err)
	}

	log.Println("Listening on:", addr)
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Fatalln("Failed to accept socket:", err)
		}

		go server.Handle(conn)
	}
}
