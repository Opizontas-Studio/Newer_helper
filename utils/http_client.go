package utils

import (
	"net"
	"net/http"
	"time"
)

var (
	// GlobalHTTPClient is a shared HTTP client with sane defaults.
	GlobalHTTPClient *http.Client
)

func init() {
	// Create a custom transport with connection pooling and timeouts
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConnsPerHost:   10, // Limit idle connections per host
	}

	// Create the global HTTP client
	GlobalHTTPClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second, // Overall request timeout
	}
}
