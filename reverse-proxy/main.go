package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

var slugPattern = regexp.MustCompile(`^[a-z]+-[a-z]+-[a-z]+$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type SubDomainResponse struct {
	DeploymentId string `json:"deploymentId"`
}

// resolveCache caches slug -> deployment ID lookups so the edge does not hit
// the API on every request. Negative results are cached briefly too, so
// requests for unknown slugs cannot hammer the API.
type resolveCache struct {
	mu       sync.Mutex
	entries  map[string]cacheEntry
	positive time.Duration
	negative time.Duration
}

type cacheEntry struct {
	deploymentID string // empty means "known not to exist"
	expiresAt    time.Time
}

func newResolveCache(positive, negative time.Duration) *resolveCache {
	return &resolveCache{
		entries:  make(map[string]cacheEntry),
		positive: positive,
		negative: negative,
	}
}

func (c *resolveCache) get(slug string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[slug]
	if !ok || time.Now().After(entry.expiresAt) {
		delete(c.entries, slug)
		return "", false
	}
	return entry.deploymentID, true
}

func (c *resolveCache) put(slug, deploymentID string) {
	ttl := c.positive
	if deploymentID == "" {
		ttl = c.negative
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[slug] = cacheEntry{deploymentID: deploymentID, expiresAt: time.Now().Add(ttl)}
}

type serveDeps struct {
	resolver     OriginResolver
	client       *http.Client
	apiServerURL string
	proxyToken   string
	siteDomain   string
	cache        *resolveCache
	port         string
}

func main() {
	godotenv.Load()

	deps, err := loadDeps(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr: ":" + deps.port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serve(w, r, *deps)
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	fmt.Printf("Server is running on port %s\n", deps.port)
	log.Fatal(server.ListenAndServe())
}

func loadDeps(getenv func(string) string) (*serveDeps, error) {
	required := func(name string) (string, error) {
		v := getenv(name)
		if v == "" {
			return "", fmt.Errorf("%s must be set", name)
		}
		return v, nil
	}

	port := getenv("PORT")
	if port == "" {
		port = "8000"
	}
	apiServerURL, err := required("API_SERVER_URL")
	if err != nil {
		return nil, err
	}
	proxyToken, err := required("PROXY_SERVICE_TOKEN")
	if err != nil {
		return nil, err
	}
	siteDomain, err := required("SITE_DOMAIN")
	if err != nil {
		return nil, err
	}

	resolver, err := NewS3OriginResolver(getenv("ARTIFACT_BASE_URL"))
	if err != nil {
		return nil, err
	}

	return &serveDeps{
		resolver:     resolver,
		client:       &http.Client{Timeout: 5 * time.Second},
		apiServerURL: strings.TrimRight(apiServerURL, "/"),
		proxyToken:   proxyToken,
		siteDomain:   strings.ToLower(siteDomain),
		cache:        newResolveCache(30*time.Second, 10*time.Second),
		port:         port,
	}, nil
}

// hostLabel extracts the subdomain label from a request host and verifies the
// host actually belongs to the configured site domain, so the proxy never
// serves content for arbitrary hostnames pointed at it.
func hostLabel(host, siteDomain string) (string, error) {
	host = strings.ToLower(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(host, ".")

	suffix := "." + siteDomain
	if !strings.HasSuffix(host, suffix) {
		return "", fmt.Errorf("host %q is not under %q", host, siteDomain)
	}
	label := strings.TrimSuffix(host, suffix)
	if label == "" || strings.Contains(label, ".") {
		return "", fmt.Errorf("host %q has an invalid subdomain label", host)
	}
	return label, nil
}

func serve(w http.ResponseWriter, r *http.Request, d serveDeps) {
	label, err := hostLabel(r.Host, d.siteDomain)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	deploymentID, err := d.lookupDeployment(label)
	if err != nil {
		http.Error(w, "Site not found", http.StatusNotFound)
		return
	}

	target, err := d.resolver.Resolve(deploymentID)
	if err != nil {
		log.Printf("Error resolving origin URL: %v", err)
		http.Error(w, "Failed to resolve origin URL", http.StatusInternalServerError)
		return
	}

	r.URL.Path = staticPath(r.URL.Path)
	proxyFor(target).ServeHTTP(w, r)
}

// lookupDeployment maps a subdomain label to a deployment ID. Slug labels go
// through the resolve API; bare deployment IDs (used for immutable per-deploy
// URLs) are accepted directly after shape validation.
func (d *serveDeps) lookupDeployment(label string) (string, error) {
	if uuidPattern.MatchString(strings.ToLower(label)) && !slugPattern.MatchString(label) {
		return strings.ToLower(label), nil
	}
	if !slugPattern.MatchString(label) {
		return "", fmt.Errorf("unrecognized subdomain %q", label)
	}
	if id, ok := d.cache.get(label); ok {
		if id == "" {
			return "", fmt.Errorf("no deployment cached for slug %q", label)
		}
		return id, nil
	}

	id, err := resolveDeploymentID(d.client, d.apiServerURL, d.proxyToken, label)
	if err != nil {
		d.cache.put(label, "")
		return "", err
	}
	d.cache.put(label, id)
	return id, nil
}

// staticPath maps URL paths onto S3 object keys using standard static-site
// conventions: directory URLs get index.html, extension-less paths fall back
// to index.html for SPA client-side routing, real files pass through.
func staticPath(path string) string {
	switch {
	case path == "" || path == "/":
		return "/index.html"
	case strings.HasSuffix(path, "/"):
		return path + "index.html"
	case strings.Contains(lastSegment(path), "."):
		return path
	default:
		return "/index.html"
	}
}

func lastSegment(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func resolveDeploymentID(client *http.Client, apiServerURL, proxyToken, slug string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, apiServerURL+"/resolve/"+url.PathEscape(slug), nil)
	if err != nil {
		return "", fmt.Errorf("failed to build resolve request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+proxyToken)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve API unreachable: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("unknown slug %q", slug)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve API returned status %d", resp.StatusCode)
	}

	var response SubDomainResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode resolve response: %w", err)
	}
	if response.DeploymentId == "" {
		return "", fmt.Errorf("no deployment ID found for slug %q", slug)
	}
	return response.DeploymentId, nil
}
