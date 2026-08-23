package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
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
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + deploymentID + "/"
	return &u, nil
}

var proxyCache sync.Map // full origin identity -> *httputil.ReverseProxy

func proxyFor(target *url.URL) *httputil.ReverseProxy {
	key := target.Scheme + "://" + target.Host + target.Path
	if cached, ok := proxyCache.Load(key); ok {
		return cached.(*httputil.ReverseProxy)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	ogDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		ogDirector(req)
		req.Host = target.Host
		req.Header.Set("Host", target.Host)
	}
	actual, _ := proxyCache.LoadOrStore(key, proxy)
	return actual.(*httputil.ReverseProxy)
}
