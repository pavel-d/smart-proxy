slt is a dead-simple TLS reverse-proxy with SNI multiplexing (TLS virtual hosts).

That means you can send TLS/SSL connections for multiple different applications to the same port and forward
them all to the appropriate backend hosts depending on the intended destination.

# Features

### SNI Multiplexing
slt multiplexes connections to a single TLS port by inspecting the name in the SNI extension field of each connection.

### Simple YAML Configuration
You configure slt with a simple YAML configuration file:

    bind_addr: ":443"

    upstreams:
      - example.com


# Running it
Running slt is also simple. It takes a single argument, the path to the configuration file:

    ./smart-proxy /path/to/config.yml


# Building it
Just cd into the directory and "go build". It requires Go 1.1+. Or use official docker image

    docker run --rm -v "$PWD":/usr/src/smart-proxy -w /usr/src/smart-proxy golang:1.8  /bin/bash -c "go get -d ./... && go build -v"

# Testing it
Just cd into the directory and "go test".

# Stability
I run slt in production handling hundreds of thousands of connections daily.

# License
Apache
