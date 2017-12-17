package proxy

import (
	"errors"
	"sync"

	"github.com/sirupsen/logrus"
	tomb "gopkg.in/tomb.v1"

	"net"
)

// Client <-> proxy <-> Upstream
type Proxy struct {
	sync.Mutex

	Name     string `json:"name"`
	Listen   string `json:"listen"`
	Upstream string `json:"upstream"`
	Enabled  bool   `json:"enabled"`

	started chan error

	tomb        tomb.Tomb
	connections ConnectionList
}

type ConnectionList struct {
	list map[string]net.Conn
	lock sync.Mutex
}

func (c *ConnectionList) Lock() {
	c.lock.Lock()
}

func (c *ConnectionList) Unlock() {
	c.lock.Unlock()
}

var ErrProxyAlreadyStarted = errors.New("Proxy already started")

func NewProxy() *Proxy {
	proxy := &Proxy{
		started:     make(chan error),
		connections: ConnectionList{list: make(map[string]net.Conn)},
	}
	return proxy
}

func (proxy *Proxy) Start() error {
	proxy.Lock()
	defer proxy.Unlock()

	return start(proxy)
}

func (proxy *Proxy) Update(input *Proxy) error {
	proxy.Lock()
	defer proxy.Unlock()

	if input.Listen != proxy.Listen || input.Upstream != proxy.Upstream {
		stop(proxy)
		proxy.Listen = input.Listen
		proxy.Upstream = input.Upstream
	}

	if input.Enabled != proxy.Enabled {
		if input.Enabled {
			return start(proxy)
		}
		stop(proxy)
	}
	return nil
}

func (proxy *Proxy) Stop() {
	proxy.Lock()
	defer proxy.Unlock()

	stop(proxy)
}

func (proxy *Proxy) server() {
	ln, err := net.Listen("tcp", proxy.Listen)
	if err != nil {
		proxy.started <- err
		return
	}

	proxy.Listen = ln.Addr().String()
	proxy.started <- nil

	logrus.WithFields(logrus.Fields{
		"name":     proxy.Name,
		"proxy":    proxy.Listen,
		"upstream": proxy.Upstream,
	}).Info("Started proxy")

	acceptTomb := tomb.Tomb{}
	defer acceptTomb.Done()

	go func() {
		<-proxy.tomb.Dying()

		acceptTomb.Killf("Shutting down from stop()")
		// Unblock ln.Accept()
		err := ln.Close()
		if err != nil {
			acceptTomb.Done()
			logrus.WithFields(logrus.Fields{
				"proxy":  proxy.Name,
				"listen": proxy.Listen,
				"err":    err,
			}).Warn("Attempted to close an already closed proxy server")
		}

		// Wait for the accept loop to finish processing
		acceptTomb.Wait()
		proxy.tomb.Done()
	}()

	for {
		client, err := ln.Accept()
		if err != nil {
			// See http://zhen.org/blog/graceful-shutdown-of-go-net-dot-listeners/
			select {
			case <-acceptTomb.Dying():
			default:
				logrus.WithFields(logrus.Fields{
					"proxy":  proxy.Name,
					"listen": proxy.Listen,
					"err":    err,
				}).Warn("Error while accepting client")
			}
			return
		}

		logrus.WithFields(logrus.Fields{
			"name":     proxy.Name,
			"client":   client.RemoteAddr(),
			"proxy":    proxy.Listen,
			"upstream": proxy.Upstream,
		}).Info("Accepted client")

		upstream, err := net.Dial("tcp", proxy.Upstream)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"name":     proxy.Name,
				"client":   client.RemoteAddr(),
				"proxy":    proxy.Listen,
				"upstream": proxy.Upstream,
			}).Error("Unable to open connection to upstream")
			client.Close()
			continue
		}

		name := client.RemoteAddr().String()
		proxy.connections.Lock()
		proxy.connections.list[name+"upstream"] = upstream
		proxy.connections.list[name+"downstream"] = client
		proxy.connections.Unlock()
		// TODO: Start a link <> name+"upstream", client, upstream : proxy.Upstream
		// TODO: Start a link <> upstream, client, name+"downstream": proxy.Downstream
	}
}

func (proxy *Proxy) RemoveConnection(name string) {
	proxy.connections.Lock()
	defer proxy.connections.Unlock()
	delete(proxy.connections.list, name)
}

func start(proxy *Proxy) error {
	if proxy.Enabled {
		return ErrProxyAlreadyStarted
	}

	proxy.tomb = tomb.Tomb{} // Reset tomb, from previous starts/stops
	go proxy.server()
	err := <-proxy.started
	proxy.Enabled = err == nil
	return err
}

func stop(proxy *Proxy) {
	if !proxy.Enabled {
		return
	}
	proxy.Enabled = false

	proxy.tomb.Killf("Shutting down from stop()")
	proxy.tomb.Wait() // Wait until we stop accepting nenections

	proxy.connections.Lock()
	defer proxy.connections.Unlock()
	for _, conn := range proxy.connections.list {
		conn.Close()
	}

	logrus.WithFields(logrus.Fields{
		"name":     proxy.Name,
		"proxy":    proxy.Listen,
		"upstream": proxy.Upstream,
	}).Info("Terminated proxy")
}