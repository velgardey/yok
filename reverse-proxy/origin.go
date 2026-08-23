package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

type OriginResolver interface {
	Resolve(deploymentID string) (*url.URL, error)
}

type s3OriginResolver struct {
	baseURL *url.URL
}

func NewS3OriginResolver(base string) (OriginResolver, error) {
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("invalid ARTIFACT_BASE_URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("ARTIFACT_BASE_URL must be absolute, got %q", base)
	}
	return &s3OriginResolver{baseURL: u}, nil
}

func (s *s3OriginResolver) Resolve(deploymentID string) (*url.URL, error) {
	u := *s.baseURL
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + url.PathEscape(deploymentID) + "/"
	return &u, nil
}

var proxyCache sync.Map // full origin identity -> *httputil.ReverseProxy

// originTransport bounds time spent connecting and waiting for upstream
// response headers while leaving response-body streaming uncapped.
var originTransport = &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
	TLSHandshakeTimeout:   5 * time.Second,
	ResponseHeaderTimeout: 15 * time.Second,
	IdleConnTimeout:       90 * time.Second,
}

func proxyFor(target *url.URL) *httputil.ReverseProxy {
	key := target.Scheme + "://" + target.Host + target.Path
	if cached, ok := proxyCache.Load(key); ok {
		return cached.(*httputil.ReverseProxy)
	}
	// NewSingleHostReverseProxy's default director already rewrites the URL to
	// the target origin; we only need bounded upstream timeouts.
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = originTransport
	actual, _ := proxyCache.LoadOrStore(key, proxy)
	return actual.(*httputil.ReverseProxy)
}
