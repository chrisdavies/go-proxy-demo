// The http handler for proxying requests
package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
)

// The special port that we proxy differently
var specialPort = ":4443"

// RuleProvder looks up proxy and redirect rules based on a request
type RuleProvder interface {
	GetProxyRule(r *http.Request) (string, error)
	GetRedirectRule(r *http.Request) (string, error)
}

// Proxy controls the proxying / redirecting of requests to v1 or v2
type Proxy struct {
	*Config
	*httputil.ReverseProxy
	RuleProvder
}

// NewProxy initializes a proxy instance.
func NewProxy(config *Config, rules RuleProvder) *Proxy {
	proxy := &Proxy{
		ReverseProxy: &httputil.ReverseProxy{ErrorHandler: proxyErrorHandler},
		Config:       config,
		RuleProvder:  rules,
	}

	// It's possible that we want to ignore TLS errors on the destination
	// being proxied (e.g. in test environments)
	if config.IgnoreTLSErrors {
		proxy.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return proxy
}

// prepRequestForProxy configures the request headers and host for proxying
func prepRequestForProxy(r *http.Request, toDomain string) *http.Request {
	r.URL.Scheme = "https"
	r.Header.Set("X-Forwarded-Host", r.Host)
	r.Header.Set("X-Forwarded-Proto", "https")
	r.URL.Host = toDomain
	return r
}

// ServeHTTP proxies requests to v1 or v2, or redirects them to new URLs based
// on redirect rulres.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Default().Println("[info]", r.Method, r.Host, r.URL.String())

	// If we get a request on specialPort, it's always a v1 request
	if strings.HasSuffix(r.Host, specialPort) {
		prepRequestForProxy(r, p.V1Host+specialPort)
		p.ReverseProxy.ServeHTTP(w, r)
		return
	}

	// See if the requested domain has a proxy rule, and if so, proxy to the
	// the specified new domain.
	toDomain, err := p.GetProxyRule(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Proxy error"))
		log.Println("[error][GetProxyRule]", err)
		return
	}
	if toDomain != "" {
		p.ReverseProxy.ServeHTTP(w, prepRequestForProxy(r, toDomain))
		return
	}

	// See if any redirect rules match this request, and if so, redirect to
	// the specified new URL.
	toUrl, err := p.GetRedirectRule(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Proxy error"))
		log.Println("[error][GetRedirectRule]", err)
		return
	}
	if toUrl != "" {
		http.Redirect(w, r, toUrl, http.StatusTemporaryRedirect)
		return
	}

	// No proxy or redirect rules apply, so we default to proxying to v1
	p.ReverseProxy.ServeHTTP(w, prepRequestForProxy(r, p.V1Host))
}
