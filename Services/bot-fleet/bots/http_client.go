// Package bots implements the simulated trading bot personas.
package bots

import (
	"net"
	"net/http"
	"time"
)

// SharedTransport returns a tuned HTTP transport shared by all bots targeting
// the same contestant container. Reusing one transport keeps the connection
// pool warm and avoids exhausting file descriptors at high bot counts.
func SharedTransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   2 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   200,
		MaxConnsPerHost:       200,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   2 * time.Second,
		DisableCompression:    true,
		ForceAttemptHTTP2:     false,
		ExpectContinueTimeout: time.Second,
	}
}
