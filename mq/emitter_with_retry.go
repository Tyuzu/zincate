package mq

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/quic-go/quic-go/http3"
)

var index_url = os.Getenv("SQLITE_INDEX_URL")

type Index struct {
	EntityType string `json:"entity_type"`
	Action     string `json:"action"`
	EntityId   string `json:"entity_id"`
	ItemId     string `json:"item_id"`
	ItemType   string `json:"item_type"`
}

// func Emit(eventName string, content Index) error {
// 	fmt.Println(eventName, content)
// 	return nil
// }

func Emit(eventName string, content Index) error {
	fmt.Println(eventName, " emitted")

	// Convert `Index` struct to JSON
	jsonData, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %v", err)
	}

	// Send to QUIC server with retry logic
	err = QUIClient(index_url, jsonData)
	if err != nil {
		return fmt.Errorf("error sending data to QUIC server: %v", err)
	}

	return nil
}

func QUIClient(url string, jsonData []byte) error {
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

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("error creating request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Attempt %d: request failed: %v", attempt, err)

			if attempt < maxRetries {
				waitTime := baseDelay * (1 << (attempt - 1)) // Exponential backoff
				log.Printf("Retrying in %v...", waitTime)
				time.Sleep(waitTime)
				continue
			}
			return fmt.Errorf("request failed after %d attempts: %v", maxRetries, err)
		}

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading response: %v", err)
		}

		// Track memory after request
		var memAfter runtime.MemStats
		runtime.ReadMemStats(&memAfter)

		// Calculate execution time & memory usage
		elapsed := time.Since(start)
		memUsed := memAfter.Alloc - memBefore.Alloc

		fmt.Printf("Server Response: %s\n", string(body))
		fmt.Printf("Execution Time: %v\n", elapsed)
		fmt.Printf("Memory Used: %d bytes\n", memUsed)

		return nil // Success, no need to retry
	}

	return fmt.Errorf("request failed after %d attempts", maxRetries)
}

func Notify(eventName string, content Index) error {
	fmt.Println(eventName, " Notified")
	return nil
}

// event : eventid, userid, action
// ticket : ticketid, eventid, action
// merch : merchid, eventid, action
// media : mediaid, userid, action
// place : palceid, userid, action
// menu : placeid, menuid, action
// reviews : reviewid, postid, action
// feedpost : postid, userid, action
// users : userid, username, action
// settings : settingsid, userid, action
// follows : giver, reciever, action
// hashtags : tagid, postid, posttype
// likes : postid, userid, action
// comments : postid, commentid, posttype
