package main

import (
	"fmt"
	"github.com/pavel-d/smart-proxy/proxy"
	"github.com/pavel-d/smart-proxy/traf_counter"
	"github.com/pavel-d/smart-proxy/util"
	"gopkg.in/redis.v3"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
)

var Redis *redis.Client
var AuthBackendHost string
var TrafCounter *traf_counter.TrafCounter

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

	// AuthBackendHost = os.Getenv("AUTH_BACKEND_HOST")

	// if AuthBackendHost == "" {
	// 	fmt.Println("AUTH_BACKEND_HOST env var is not set")
	// 	os.Exit(1)
	// }

	// Init redis
	Redis = newRedisClient()
	TrafCounter = &traf_counter.TrafCounter{Redis}

	var completed sync.WaitGroup
	completed.Add(len(config.ListenersConfig))

	for _, listener := range config.ListenersConfig {

		// run server
		proxyServer := &proxy.Server{
			Configuration:  config,
			Logger:         log.New(os.Stdout, "smart-proxy ", log.LstdFlags|log.Lshortfile),
			ListenerConfig: listener,
			Interceptor:    getInterceptor(Redis, &proxy.Backend{AuthBackendHost, proxy.DefaultConnectTimeout}),
			TrafCounter:    TrafCounter,
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

func newRedisClient() *redis.Client {
	redisCLient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       1,
	})

	return redisCLient
}

func getInterceptor(redisClient *redis.Client, authBackend *proxy.Backend) proxy.Interceptor {
	return func(c net.Conn, front *proxy.Frontend, back *proxy.Backend) *proxy.Backend {

		// ipAddr := util.IPAddrFromRemoteAddr(c.RemoteAddr())

		// result, _ := redisClient.Get(ipAddr).Result()

		// if result == "" {
		// 	log.Println("Anonymous")
		// 	return authBackend
		// } else {
		// 	log.Println("Authenticated")
		// }

		return back
	}
}
