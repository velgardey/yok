package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type SubDomainResponse struct {
	DeploymentId string `json:"deploymentId"`
}

func main() {
	godotenv.Load()

	//Get Environment Variables
	PORT := os.Getenv("PORT")
	bucketName := os.Getenv("AWS_S3_BUCKET")
	region := os.Getenv("AWS_REGION")
	apiServerUrl := os.Getenv("API_SERVER_URL")

	//Generate base path for S3
	basePath := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/__output/", bucketName, region)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hostName := r.Host
		// Get the subdomain/slug from the host name
		parts := strings.Split(hostName, ".")
		subDomain := parts[0]
		deploymentId := subDomain

		// Validate the slug pattern and check if the deployment ID is being fetched from the API server
		var slugPattern = regexp.MustCompile(`^[a-z]+-[a-z]+-[a-z]+$`)
		if slugPattern.MatchString(subDomain) {
			apiUrl := fmt.Sprintf("%s/resolve/%s", apiServerUrl, subDomain)
			log.Printf("Resolving deployment ID for subdomain: %s", subDomain)

			resp, err := client.Get(apiUrl)
			if err != nil {
				log.Printf("Error resolving deployment ID: %v", err)
				http.Error(w, "Failed to receive deployment Id", http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()

			log.Printf("Response status: %v", resp.StatusCode)

			if resp.StatusCode != http.StatusOK {
				log.Printf("Error resolving deployment ID: %v", resp.StatusCode)
				http.Error(w, "Failed to receive deployment Id", http.StatusInternalServerError)
				return
			}

			//Read the response body with the deployment ID
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				http.Error(w, "Failed to read response body for deployment ID", http.StatusInternalServerError)
				return
			}
			log.Printf("Response body: %s", string(body))

			var response SubDomainResponse
			if err := json.Unmarshal(body, &response); err != nil {
				log.Printf("Error unmarshalling response body: %v", err)
				http.Error(w, "Failed to unmarshal response body for deployment ID", http.StatusInternalServerError)
				return
			}
			log.Printf("Deployment ID: %s", response.DeploymentId)
			if response.DeploymentId == "" {
				log.Printf("No deployment ID found for subdomain: %s", subDomain)
				http.Error(w, "No deployment ID found", http.StatusNotFound)
				return
			}
			deploymentId = response.DeploymentId
		}

		// Construct the S3 URL for the deployment
		resolvesTo := basePath + deploymentId + "/"
		log.Printf("Resolves to: %s", resolvesTo)
		targetUrl, err := url.Parse(resolvesTo)
		if err != nil {
			log.Printf("Error parsing resolvesTo URL: %v", err)
			http.Error(w, "Failed to parse resolvesTo URL", http.StatusInternalServerError)
			return
		}

		//Properly append index.html to the URL
		urlPath := r.URL.Path
		if urlPath == "/" || urlPath == "" {
			urlPath = "/index.html"
			r.URL.Path = urlPath
		}

		// Check if the assets folder is nested or not and resolve it
		pathRegex := regexp.MustCompile(`^/([^/]+)/(.*)$`)
		pathMatch := pathRegex.FindStringSubmatch(urlPath)

		if pathMatch != nil {
			firstSegment := pathMatch[1]
			remainingPath := pathMatch[2]
			knownAssetDirs := map[string]bool{
				"assets": true,
				"images": true,
				"static": true,
				"media":  true,
				"_next":  true,
				"js":     true,
				"css":    true,
			}
			if !knownAssetDirs[firstSegment] {
				r.URL.Path = "/" + remainingPath
				log.Printf("Rewriting path from %s to %s", urlPath, r.URL.Path)
			}
		}

		// Create a reverse proxy to the target URL
		proxy := httputil.NewSingleHostReverseProxy(targetUrl)

		ogDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			ogDirector(req)
			req.Host = targetUrl.Host
			req.Header.Set("Host", targetUrl.Host)
		}
		proxy.ServeHTTP(w, r)
	})
	fmt.Printf("Server is running on port %s\n", PORT)
	log.Fatal(http.ListenAndServe(":"+PORT, nil))
}
