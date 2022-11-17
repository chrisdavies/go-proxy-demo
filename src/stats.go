// Track proxy stats
package main

import (
	"net/http"
	"strconv"
	"sync/atomic"
)

// Stats tracks request and connection stats for the life of the service
type Stats struct {
	openConnections int64
}

// ServeHTTP serves the connection statistics (unsecured)
func (stats *Stats) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.Write([]byte("<pre>Connections " + strconv.FormatInt(stats.openConnections, 10) + "</pre>"))
}

// IncConnections increments or decrements the number of open connections
func (stats *Stats) IncConnections(amount int64) {
	atomic.AddInt64(&stats.openConnections, amount)
}
