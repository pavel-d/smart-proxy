package proxy

import (
	"crypto/tls"
	vhost "github.com/inconshreveable/go-vhost"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

const (
	muxTimeout = 10 * time.Second
)

type LoadTLSConfigFn func(crtPath, keyPath string) (*tls.Config, error)

type Options struct {
	ConfigPath string
}

type Backend struct {
	Addr           string `"yaml:addr"`
	ConnectTimeout int    `yaml:connect_timeout"`
}

type Frontend struct {
	Backends []Backend `yaml:"backends"`
	Strategy string    `yaml:"strategy"`
	TLSCrt   string    `yaml:"tls_crt"`
	TLSKey   string    `yaml:"tls_key"`
	Default  bool      `yaml:"default"`

	strategy  BackendStrategy `yaml:"-"`
	TlsConfig *tls.Config     `yaml:"-"`
}

type Configuration struct {
	ListenersConfig []ListenerConfig     `yaml:"listeners"`
	Frontends       map[string]*Frontend `yaml:"frontends"`
	DefaultFrontend *Frontend
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

func (s *Server) Run() error {
	// bind a port to handle TLS/HTTP connections
	l, err := net.Listen("tcp", s.ListenerConfig.BindAddr)
	if err != nil {
		return err
	}
	s.Printf("Serving connections on %v", l.Addr())

	if s.ListenerConfig.Https {
		s.Logger.Println("Initializing HTTPS multiplexer")
		s.mux, err = vhost.NewTLSMuxer(l, muxTimeout)
	} else {
		s.Logger.Println("Initializing HTTP multiplexer")
		s.mux, err = vhost.NewHTTPMuxer(l, muxTimeout)
	}

	if err != nil {
		return err
	}

	// wait for all frontends to finish
	s.wait.Add(len(s.Frontends))

	// setup muxing for each frontend
	for name, front := range s.Frontends {
		fl, err := s.mux.Listen(name)

		if err != nil {
			return err
		}
		go s.runFrontend(name, front, fl)
	}

	// custom error handler so we can log errors
	go func() {
		for {
			conn, err := s.mux.NextError()
			if conn == nil {
				s.Printf("Failed to mux next connection, error: %v", err)
				if _, ok := err.(vhost.Closed); ok {
					return
				} else {
					continue
				}
			} else {
				if _, ok := err.(vhost.NotFound); ok && s.DefaultFrontend != nil {
					go s.proxyConnection(conn, s.DefaultFrontend)
				} else {
					s.Printf("Failed to mux connection from %v, error: %v", conn.RemoteAddr(), err)
					// XXX: respond with valid TLS close messages
					conn.Close()
				}
			}
		}
	}()

	// we're ready, signal it for testing
	if s.ready != nil {
		close(s.ready)
	}

	s.wait.Wait()

	return nil
}

func (s *Server) runFrontend(name string, front *Frontend, l net.Listener) {
	// mark finished when done so Run() can return
	defer s.wait.Done()

	// always round-robin strategy for now
	front.strategy = &RoundRobinStrategy{backends: front.Backends}

	s.Printf("Handling connections to %v", name)
	for {
		// accept next connection to this frontend
		conn, err := l.Accept()
		if err != nil {
			s.Printf("Failed to accept new connection for '%v': %v", conn.RemoteAddr())
			if e, ok := err.(net.Error); ok {
				if e.Temporary() {
					continue
				}
			}
			return
		}

		s.Printf("Accepted new connection for %v from %v", name, conn.RemoteAddr())

		// proxy the connection to an backend
		go s.proxyConnection(conn, front)
	}
}

func (s *Server) proxyConnection(c net.Conn, front *Frontend) (err error) {
	// unwrap if tls cert/key was specified
	if front.TlsConfig != nil {
		c = tls.Server(c, front.TlsConfig)
	}

	// pick the backend
	backend := front.strategy.NextBackend()
	// dial the backend
	upConn, err := net.DialTimeout("tcp", backend.Addr+":"+s.ListenerConfig.BindPort, time.Duration(backend.ConnectTimeout)*time.Millisecond)
	if err != nil {
		s.Printf("Failed to dial backend connection %v: %v", backend.Addr, err)
		c.Close()
		return
	}
	s.Printf("Initiated new connection to backend: %v %v", upConn.LocalAddr(), upConn.RemoteAddr())

	// join the connections
	s.joinConnections(c, upConn)
	return
}

func (s *Server) joinConnections(c1 net.Conn, c2 net.Conn) {
	var wg sync.WaitGroup
	halfJoin := func(dst net.Conn, src net.Conn) {
		defer wg.Done()
		defer dst.Close()
		defer src.Close()
		n, err := io.Copy(dst, src)
		s.Printf("Copy from %v to %v failed after %d bytes with error %v", src.RemoteAddr(), dst.RemoteAddr(), n, err)
	}

	s.Printf("Joining connections: %v %v", c1.RemoteAddr(), c2.RemoteAddr())
	wg.Add(2)
	go halfJoin(c1, c2)
	go halfJoin(c2, c1)
	wg.Wait()
}

type BackendStrategy interface {
	NextBackend() Backend
}

type RoundRobinStrategy struct {
	backends []Backend
	idx      int
}

func (s *RoundRobinStrategy) NextBackend() Backend {
	n := len(s.backends)

	if n == 1 {
		return s.backends[0]
	} else {
		s.idx = (s.idx + 1) % n
		return s.backends[s.idx]
	}
}

func LoadTLSConfig(crtPath, keyPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(crtPath, keyPath)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}
