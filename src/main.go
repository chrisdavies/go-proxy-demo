// This application proxies requests to v1 and v2 with the intent of making it
// easy to switch a domain or content from one to the other with no downtime.
//
// It listens on systemd sockets, then forwards any connections on those
// sockets to certmagic.
//
// To run it in development, use ../run-dev.sh
//
// In production, systemd will control ports 80 and 443 and forward them to
// this service. This service will listen on the file descriptors that systemd
// indicates (via environment variables) and will *also* listen on two local
// ports: 8080 for http and 4433 for https. We will proxy the systemd
// connections to the local ports.
//

package main

import (
	"log"
	"net/http"
	"strings"
)

func main() {
	config, err := NewConfig()

	if err != nil {
		log.Fatalln("Invalid config:", err)
	}

	log.Println("[info]", "[dev =", config.IsDev, "]", "Starting with settings:", "V1_HOST=", config.V1Host, "IGNORE_TLS=", config.IgnoreTLSErrors)

	admin, err := AdminService(config)
	if err != nil {
		log.Fatalln("Failed to initialize admin service", err)
	}

	stats := &Stats{}
	proxy := NewProxy(config, admin)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/goproxy/proxy-rules"):
			admin.ProxyRules(w, r)
		case strings.HasPrefix(r.URL.Path, "/goproxy/redirect-rules"):
			admin.RedirectRules(w, r)
		case strings.HasPrefix(r.URL.Path, "/goproxy"):
			stats.ServeHTTP(w, r)
		default:
			proxy.ServeHTTP(w, r)
		}
	})

	startServer(config, stats)
}
