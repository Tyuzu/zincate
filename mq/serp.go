package mq

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/quic-go/quic-go/http3"
)

var serp_url = os.Getenv("SERP_URL")

// FetchEvents retrieves all events from the QUIC server
func FetchResults(entityType string, query string) ([]byte, error) {
	url := serp_url
	client := &http.Client{
		Transport: &http3.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skip verification for self-signed cert
		},
	}

	// Retry logic: 3 attempts with exponential backoff
	maxRetries := 3
	baseDelay := time.Second // 1s initial delay

	for attempt := 1; attempt <= maxRetries; attempt++ {
		start := time.Now() // Start profiling time

		// Track memory before request
		var memBefore runtime.MemStats
		runtime.ReadMemStats(&memBefore)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return []byte{}, fmt.Errorf("error creating request: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Attempt %d: request failed: %v", attempt, err)

			if attempt < maxRetries {
				waitTime := baseDelay * (1 << (attempt - 1)) // Exponential backoff
				log.Printf("Retrying in %v...", waitTime)
				time.Sleep(waitTime)
				continue
			}
			return []byte{}, fmt.Errorf("request failed after %d attempts: %v", maxRetries, err)
		}

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return []byte{}, fmt.Errorf("error reading response: %v", err)
		}

		// Track memory after request
		var memAfter runtime.MemStats
		runtime.ReadMemStats(&memAfter)

		// Calculate execution time & memory usage
		elapsed := time.Since(start)
		memUsed := memAfter.Alloc - memBefore.Alloc

		// fmt.Printf("Server Response: %s\n", string(body))
		fmt.Printf("Execution Time: %v\n", elapsed)
		fmt.Printf("Memory Used: %d bytes\n", memUsed)

		return body, nil // Success, no need to retry
	}

	return []byte{}, fmt.Errorf("request failed after %d attempts", maxRetries)
}
