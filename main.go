package main

import (
	"fmt"
	"github.com/pavel-d/smart-proxy/proxy"
	"github.com/pavel-d/smart-proxy/util"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
)

func main() {
	// parse command line options
	opts, err := util.ParseArgs()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// read configuration file
	configBuf, err := ioutil.ReadFile(opts.ConfigPath)
	if err != nil {
		fmt.Printf("Failed to read configuration file %s: %v\n", opts.ConfigPath, err)
		os.Exit(1)
	}

	// parse configuration file
	config, err := util.ParseConfig(configBuf, proxy.LoadTLSConfig)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var completed sync.WaitGroup
	completed.Add(len(config.ListenersConfig))

	for _, listener := range config.ListenersConfig {

		// run server
		proxyServer := &proxy.Server{
			Configuration:  config,
			Logger:         log.New(os.Stdout, "unlocker-proxy ", log.LstdFlags|log.Lshortfile),
			ListenerConfig: listener,
			Middleware:     interceptor,
		}
		// this blocks unless there's a startup error
		go func(server *proxy.Server) {
			err = server.Run()
			if err != nil {
				fmt.Printf("Failed to start server %s: %v\n", listener, err)
			}
			completed.Done()
		}(proxyServer)
	}

	completed.Wait()
}

func interceptor(c net.Conn, front *proxy.Frontend, back *proxy.Backend) *proxy.Backend {
	log.Printf("Hello from interceptor! %v is allowed", c.RemoteAddr())
	return back
}
