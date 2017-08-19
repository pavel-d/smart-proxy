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

func ParseConfig(configBuf []byte) (config *proxy.Configuration, err error) {
	config = new(proxy.Configuration)
	if err = goyaml.Unmarshal(configBuf, &config); err != nil {
		err = fmt.Errorf("Error parsing configuration file: %v", err)
		return
	}

	for idx, listener := range config.ListenersConfig {
		if listener.BindAddr == "" {
			err = fmt.Errorf("You must specify a bind_addr")
			return
		}
		config.ListenersConfig[idx].BindPort = strings.Split(listener.BindAddr, ":")[1]
	}

	if len(config.Upstreams) == 0 {
		err = fmt.Errorf("You must specify at least one frontend")
		return
	}
	return
}
