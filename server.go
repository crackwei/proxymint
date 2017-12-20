package main

import (
	"encoding/binary"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type (
	ClientID uint64

	ProxyServer struct {
		idCount uint64

		clients struct {
			m map[ClientID]*ProxyClient
			sync.RWMutex
		}

		// The server ID part of the nonce.
		NoncePart1a [8]byte

		Config Config
	}

	ProxyClient struct {
		ID   ClientID
		name string

		ps   *ProxyServer
		conn net.Conn
		lrw  *LRW
	}
)

// Pool timeout settings
const (
	InitTimeout = 10 * time.Second
	AuthTimeout = 10 * time.Second

	WriteTimeout = 15 * time.Second

	InactivityTimeout = 3 * time.Minute
	KeepAliveInterval = 30 * time.Second
)

func NewProxy(cfg Config) (*ProxyServer, error) {
	server := ProxyServer{
		idCount: 0,

		clients: struct {
			m map[ClientID]*ProxyClient
			sync.RWMutex
		}{
			m: make(map[ClientID]*ProxyClient),
		},

		Config: cfg,
	}

	return &server, nil
}

// Handle a new client connection, executed in a goroutine.
func (s *ProxyServer) Handle(conn *net.TCPConn) error {
	log.Println("[server] new connection from", conn.RemoteAddr())

	if err := conn.SetKeepAlive(true); err != nil {
		return err
	}

	if err := conn.SetKeepAlivePeriod(KeepAliveInterval); err != nil {
		return err
	}

	client := proxyClient{
		ID:   ClientID(atomic.AddUint64(&s.idCount, 1)),
		ps:   s,
		conn: conn,
		lrw:  NewLRW(conn),
	}

	return client.Serve()
}

// Subscribe a client onto work notifications.
func (s *ProxyServer) Subscribe(c *ProxyClient) {
	s.clients.Lock()
	s.clients.m[c.ID] = c
	s.clients.Unlock()
}

// Unsubscribe removes a client from work notifications.
func (s *ProxyServer) Unsubscribe(c *ProxyClient) {
	s.clients.Lock()
	delete(s.clients.m, c.ID)
	_ = c.Close()
	s.clients.Unlock()
}

func (s *ProxyServer) BlockNotify() error {
	log.Println("[server]", "new block detected")
	// TODO: s.getwork
	return nil
}

// Serve runs the client.
func (c *ProxyClient) Serve() (err error) {
	log.Printf("[client %v %v] -> serving\n", c.ID, c.conn.RemoteAddr())
	defer func() {
		if err != nil {
			log.Printf("[client %v %v] <-!- disconnected with error: %v\n", c.ID, c.conn.RemoteAddr(), err)
			return
		}

		log.Printf("[client %v %v] <- disconnected\n", c.ID, c.conn.RemoteAddr())
	}()

	defer c.conn.Close()

	var noncePart1 stratum.Uint128

	// Handle subscription
	subscribe, err := c.lrw.WaitForType(Subscribe, time.Now().Add(InitTimeout))
	if err != nil {
		return err
	}

	sub := subscribe.(RequestSubscribe)
	if len(sub.Params) == 2 {
		c.ps.Subscribe(c)
		defer c.ps.Unsubscribe(c)

		return c.ps.BlockNotify()
	}

	// Write in the server ID
	copy(noncePart1[:], c.ps.NoncePart1a[:])

	// Write in the client ID
	binary.LittleEndian.PutUint64(noncePart1[8:], uint64(c.ID))

	if err := c.lrw.WriteStratumTimed(ResponseSubscribeReply{
		ID:         sub.ID,
		Session:    "",
		NoncePart1: noncePart1,
	}, time.Now().Add(WriteTimeout)); err != nil {
		return err
	}

	// TODO: Handle Authorize

	log.Printf("[client %v %v] '%v'\n", c.ID, c.conn.RemoteAddr(), c.name)

	c.ps.Subscribe(c)
	defer c.ps.Unsubscribe(c)

	// Channel for reading input
	ChanRequest := make(chan Request, 64)
	go func() error {
		defer close(ChanRequest)

		for {
			req, err := c.lrw.ReadStratumTimed(time.Now().Add(InactivityTimeout))
			if err != nil {
				return err
			}

			ChanRequest <- req
		}
	}()

}

func (c *ProxyClient) Close() error {
	return c.conn.Close()
}
