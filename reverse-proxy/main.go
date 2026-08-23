package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type SubDomainResponse struct {
	DeploymentId string `json:"deploymentId"`
}

type serveDeps struct {
	resolver     OriginResolver
	client       *http.Client
	apiServerURL string
	proxyToken   string
	slugPattern  *regexp.Regexp
	assetDirs    map[string]bool
}

func main() {
	godotenv.Load()

	port := os.Getenv("PORT")
	apiServerURL := os.Getenv("API_SERVER_URL")
	proxyToken := os.Getenv("PROXY_SERVICE_TOKEN")

	if proxyToken == "" {
		log.Fatal("PROXY_SERVICE_TOKEN must be set")
	}

	resolver, err := NewS3OriginResolver(os.Getenv("ARTIFACT_BASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	deps := serveDeps{
		resolver:     resolver,
		client:       &http.Client{Timeout: 5 * time.Second},
		apiServerURL: apiServerURL,
		proxyToken:   proxyToken,
		slugPattern:  regexp.MustCompile(`^[a-z]+-[a-z]+-[a-z]+$`),
		assetDirs: map[string]bool{
			"assets": true,
			"images": true,
			"static": true,
			"media":  true,
			"_next":  true,
			"js":     true,
			"css":    true,
		},
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serve(w, r, deps)
	})
	fmt.Printf("Server is running on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func serve(w http.ResponseWriter, r *http.Request, d serveDeps) {
	hostName := r.Host
	subDomain := strings.Split(hostName, ".")[0]
	deploymentID := subDomain

	if d.slugPattern.MatchString(subDomain) {
		id, err := resolveDeploymentID(d.client, d.apiServerURL, d.proxyToken, subDomain)
		if err != nil {
			log.Printf("Error resolving deployment ID: %v", err)
			http.Error(w, "Failed to receive deployment Id", http.StatusInternalServerError)
			return
		}
		deploymentID = id
	}

	target, err := d.resolver.Resolve(deploymentID)
	if err != nil {
		log.Printf("Error resolving origin URL: %v", err)
		http.Error(w, "Failed to resolve origin URL", http.StatusInternalServerError)
		return
	}
	log.Printf("Resolves to: %s", target)

	urlPath := r.URL.Path
	if urlPath == "/" || urlPath == "" {
		r.URL.Path = "/index.html"
		urlPath = r.URL.Path
	}

	pathRegex := regexp.MustCompile(`^/([^/]+)/(.*)$`)
	pathMatch := pathRegex.FindStringSubmatch(urlPath)
	if pathMatch != nil && !d.assetDirs[pathMatch[1]] {
		r.URL.Path = "/" + pathMatch[2]
		log.Printf("Rewriting path from %s to %s", urlPath, r.URL.Path)
	}

	proxyFor(target).ServeHTTP(w, r)
}

func resolveDeploymentID(client *http.Client, apiServerURL, proxyToken, slug string) (string, error) {
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/resolve/%s", apiServerURL, slug), nil)
	req.Header.Set("Authorization", "Bearer "+proxyToken)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var response SubDomainResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response body: %w", err)
	}
	if response.DeploymentId == "" {
		return "", fmt.Errorf("no deployment ID found for slug %s", slug)
	}
	return response.DeploymentId, nil
}
