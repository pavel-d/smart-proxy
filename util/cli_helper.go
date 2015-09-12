package util

import (
	"flag"
	"fmt"
	"github.com/pavel-d/smart-proxy/proxy"
	"launchpad.net/goyaml"
	"os"
	"strings"
)

func ParseArgs() (*proxy.Options, error) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <config file>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "%s is a simple TLS reverse proxy that can multiplex TLS connections\n"+
			"by inspecting the SNI extension on each incoming connection. This\n"+
			"allows you to accept connections to many different backend TLS\n"+
			"applications on a single port.\n\n"+
			"%s takes a single argument: the path to a YAML configuration file.\n\n", os.Args[0], os.Args[0])
	}
	flag.Parse()

	if len(flag.Args()) != 1 {
		return nil, fmt.Errorf("You must specify a single argument, the path to the configuration file.")
	}

	return &proxy.Options{
		ConfigPath: flag.Arg(0),
	}, nil

}

func ParseConfig(configBuf []byte, loadTLS proxy.LoadTLSConfigFn) (config *proxy.Configuration, err error) {
	// deserialize/parse the config
	config = new(proxy.Configuration)
	if err = goyaml.Unmarshal(configBuf, &config); err != nil {
		err = fmt.Errorf("Error parsing configuration file: %v", err)
		return
	}

	// configuration validation / normalization
	for idx, listener := range config.ListenersConfig {
		if listener.BindAddr == "" {
			err = fmt.Errorf("You must specify a bind_addr")
			return
		}
		config.ListenersConfig[idx].BindPort = strings.Split(listener.BindAddr, ":")[1]
	}

	if len(config.Frontends) == 0 {
		err = fmt.Errorf("You must specify at least one frontend")
		return
	}

	for name, front := range config.Frontends {
		if len(front.Backends) == 0 {
			err = fmt.Errorf("You must specify at least one backend for frontend '%v'", name)
			return
		}

		if front.Default {
			if config.DefaultFrontend != nil {
				err = fmt.Errorf("Only one frontend may be the default")
				return
			}
			config.DefaultFrontend = front
		}

		for _, back := range front.Backends {
			if back.ConnectTimeout == 0 {
				back.ConnectTimeout = proxy.DefaultConnectTimeout
			}

			if back.Addr == "" {
				err = fmt.Errorf("You must specify an addr for each backend on frontend '%v'", name)
				return
			}
		}

		if front.TLSCrt != "" || front.TLSKey != "" {
			if front.TlsConfig, err = loadTLS(front.TLSCrt, front.TLSKey); err != nil {
				err = fmt.Errorf("Failed to load TLS configuration for frontend '%v': %v", name, err)
				return
			}
		}
	}
	return
}
