package proxy

import (
	"fmt"
	vhost "github.com/inconshreveable/go-vhost"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	muxTimeout            = 10 * time.Second
	DefaultConnectTimeout = 10000 // milliseconds
)

type Options struct {
	ConfigPath string
}

type Backend struct {
	Addr           string `"yaml:addr"`
	ConnectTimeout int    `yaml:connect_timeout"`
}

type Configuration struct {
	ListenersConfig []ListenerConfig `yaml:"listeners"`
	Upstreams       []string         `yaml:"upstreams"`
	DefaultUpstream string           `yaml:"default_upstream"`
}

type Server struct {
	*log.Logger
	*Configuration
	wait sync.WaitGroup
	// these are for easier testing
	mux            Muxer
	ready          chan int
	ListenerConfig ListenerConfig
}

type ListenerConfig struct {
	Https    bool   `yaml:"https"`
	BindAddr string `yaml:"bind_addr"`
	BindPort string
}

type Muxer interface {
	Listen(string) (net.Listener, error)
	NextError() (net.Conn, error)
}

func (server *Server) Run() error {
	// bind a port to handle TLS/HTTP connections
	l, err := net.Listen("tcp", server.ListenerConfig.BindAddr)
	if err != nil {
		return err
	}
	server.Printf("Serving connections on %v", l.Addr())

	if server.ListenerConfig.Https {
		server.Logger.Println("Initializing HTTPS multiplexer")
		server.mux, err = vhost.NewTLSMuxer(l, muxTimeout)
	} else {
		server.Logger.Println("Initializing HTTP multiplexer")
		server.mux, err = vhost.NewHTTPMuxer(l, muxTimeout)
	}

	if err != nil {
		return err
	}

	// wait for all upstreams to finish
	server.wait.Add(len(server.Configuration.Upstreams))

	// setup muxing for each frontend
	for _, host := range server.Configuration.Upstreams {
		fl, err := server.mux.Listen(host)

		if err != nil {
			return err
		}
		go server.runFrontend(host, fl)
	}

	go func() {
		for {
			conn, err := server.mux.NextError()
			if conn == nil {
				server.Printf("Failed to mux next connection, error: %v", err)
				if _, ok := err.(vhost.Closed); ok {
					return
				} else {
					continue
				}
			} else {
				if _, ok := err.(vhost.NotFound); ok && server.DefaultUpstream != "" {
					go server.proxyConnection(conn, server.DefaultUpstream)
				} else {
					server.Printf("Failed to mux connection from %v, error: %v", conn.RemoteAddr(), err)
					// TODO: respond with valid TLS close messages
					conn.Close()
				}
			}
		}
	}()

	if server.ready != nil {
		close(server.ready)
	}

	server.wait.Wait()

	return nil
}

func (server *Server) runFrontend(host string, l net.Listener) {
	// mark finished when done so Run() can return
	defer server.wait.Done()

	for {
		// accept next connection to this frontend
		conn, err := l.Accept()
		if err != nil {
			server.Printf("Failed to accept new connection for '%v': %v", conn.RemoteAddr())
			if e, ok := err.(net.Error); ok {
				if e.Temporary() {
					continue
				}
			}
			return
		}

		server.Printf("Accepted new connection for %v from %v", host, conn.RemoteAddr())

		// proxy the connection to an backend
		go server.proxyConnection(conn, host)
	}
}

func (server *Server) proxyConnection(c net.Conn, host string) (err error) {
	backend := fmt.Sprintf("%s:%s", host, server.ListenerConfig.BindPort)
	upConn, err := net.DialTimeout("tcp", backend, time.Duration(DefaultConnectTimeout)*time.Millisecond)

	if err != nil {
		server.Printf("Failed to dial backend connection %v: %v", host, err)
		c.Close()
		return
	}

	server.Printf("Initiated new connection to backend: %v %v", upConn.LocalAddr(), upConn.RemoteAddr())

	totalBytes := server.joinConnections(c, upConn)
	log.Printf("Stats: '%s' => '%s' transfered %d bytes", backend, c.RemoteAddr(), totalBytes)

	return
}

func (server *Server) joinConnections(c1 net.Conn, c2 net.Conn) int64 {
	var wg sync.WaitGroup
	var totalBytes int64

	halfJoin := func(dst net.Conn, src net.Conn) {
		defer wg.Done()
		defer dst.Close()
		defer src.Close()
		n, err := io.Copy(dst, src)
		atomic.AddInt64(&totalBytes, n)
		server.Printf("Copy from %v to %v failed after %d bytes with error %v", src.RemoteAddr(), dst.RemoteAddr(), n, err)
	}

	server.Printf("Joining connections: %v %v", c1.RemoteAddr(), c2.RemoteAddr())
	wg.Add(2)
	go halfJoin(c1, c2)
	go halfJoin(c2, c1)
	wg.Wait()
	return totalBytes
}
