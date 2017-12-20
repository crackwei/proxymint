package main

import (
	"encoding/json"
	"log"
	"net"
	"os"

	"net/http"
	_ "net/http/pprof"
)

// Config for the pool server.
type Config struct {
	Host string `json:"host"`

	PProfHost string `json:"pprof_host"`

	Testnet bool `json:"testnet"`
}

func loadConfig() (cfg Config, err error) {
	configPath := os.Getenv("CONFIG")
	if configPath == "" {
		configPath = "config.json"
	}

	file, err := os.Open(configPath)
	if err != nil {
		return cfg, err
	}

	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags | log.Lmicroseconds)

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalln("Could not load configuration:", err)
	}

	server, err := NewProxy(cfg)
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
