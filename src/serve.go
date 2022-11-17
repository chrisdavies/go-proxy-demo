// serve.go contains all of the plumbing for handling TLS, tunneling
// systemd sockets to / from the http layer, etc.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/caddyserver/certmagic"
)

// pipeConn pipes all data to / from the specified connection and the specified
// local TCP port.
func pipeConn(conn net.Conn, toPort int, stats *Stats) {
	defer conn.Close()
	stats.IncConnections(1)
	defer stats.IncConnections(-1)

	dest, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(toPort))
	if err != nil {
		log.Default().Println("[error] open tunnel:", err)
		return
	}
	defer dest.Close()
	go func() {
		if _, err := io.Copy(dest, conn); err != nil && err != net.ErrClosed {
			log.Default().Println("[error] tunnel to server:", err)
		}
	}()
	if _, err := io.Copy(conn, dest); err != nil && err != net.ErrClosed {
		log.Default().Println("[error] tunnel to client:", err)
	}
}

// tunnelSystemd listens on fd (a systemd-specified file descriptor) and
// sends all traffic to the specified "toPort" via TCP.
func tunnelSystemd(fd, toPort int, stats *Stats) {
	// Here, we open the file descriptor. Systemd file descriptors start at 3.
	listener, err := net.FileListener(os.NewFile(uintptr(3+fd), "systemd_fd_"+strconv.Itoa(fd)))
	if err != nil {
		log.Default().Fatalln(err)
	}

	// Here, we go into an infinite listen loop, accepting incoming connections
	// and piping them to the specified tcp port.
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Default().Println("[error] systemd accept", err)
			continue
		}
		go pipeConn(conn, toPort, stats)
	}
}

// proxyErrorHandler prevent logging connection closed events
func proxyErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	if r.Context().Err() != context.Canceled {
		log.Default().Println("[error] proxy", err)
	}
	w.WriteHeader(502)
}

// serveTLS runs the http server with certmagic
func serveTLS(config *Config) {
	certmagic.HTTPPort = config.HTTPPort
	certmagic.HTTPSPort = config.HTTPSPort

	// Automatic TLS for arbitrary domains so we don't need to preconfigure
	certmagic.Default.OnDemand = new(certmagic.OnDemandConfig)

	fmt.Println("[info] Listening on ports", certmagic.HTTPPort, certmagic.HTTPSPort)
	err := certmagic.HTTPS([]string{}, nil)
	if err != nil {
		log.Default().Fatalln(err)
	}
}

// serveDevMode runs the http server with vanilla Go
func serveDevMode(config *Config) {
	fmt.Println("[info][dev-mode] Listening on port", config.HTTPPort)
	err := http.ListenAndServe(":"+strconv.Itoa(config.HTTPPort), nil)
	if err != nil {
		log.Default().Fatalln(err)
	}
}

// startServer starts the http service and systemd tunnels
func startServer(config *Config, stats *Stats) {
	if config.IsDev {
		go serveDevMode(config)
	} else {
		go serveTLS(config)
	}

	// Tunnel the first systemd file descriptor to our http port, and the
	// second to our https port.
	if !config.IsDev {
		go tunnelSystemd(1, config.HTTPSPort, stats)
	}
	tunnelSystemd(0, config.HTTPPort, stats)
}
