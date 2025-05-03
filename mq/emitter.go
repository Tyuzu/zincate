package mq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"time"
)

type Index struct {
	EntityType string `json:"entity_type"`
	Method     string `json:"method"`
	EntityId   string `json:"entity_id"`
	ItemId     string `json:"item_id"`
	ItemType   string `json:"item_type"`
}

// Emit event by sending JSON data to QUIC server
func Emit(eventName string, content Index) error {
	fmt.Println(eventName, "emitted")

	jsonData, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %v", err)
	}

	err = Printer(jsonData)
	if err != nil {
		return fmt.Errorf("error sending data to QUIC server: %v", err)
	}

	return nil
}

// SERP_URL - replace with the actual URL
var SERP_URL = os.Getenv("SERP_URL") // Change to the actual endpoint

func Printer(jsonData []byte) error {
	start := time.Now()

	// Capture memory usage before request
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Send POST request
	resp, err := http.Post(SERP_URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	// Capture memory usage after request
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	elapsed := time.Since(start)
	memUsed := memAfter.Alloc - memBefore.Alloc

	fmt.Printf("Server Response: %s\n", string(body))
	fmt.Printf("Execution Time: %v\n", elapsed)
	fmt.Printf("Memory Used: %d bytes\n", memUsed)

	return nil
}

// func Printer(jsonData []byte) error {
// 	start := time.Now()

// 	var memBefore runtime.MemStats
// 	runtime.ReadMemStats(&memBefore)

// 	// Track memory after request
// 	var memAfter runtime.MemStats
// 	runtime.ReadMemStats(&memAfter)

// 	elapsed := time.Since(start)
// 	memUsed := memAfter.Alloc - memBefore.Alloc

// 	fmt.Printf("Server Response: %s\n", string(jsonData))
// 	fmt.Printf("Execution Time: %v\n", elapsed)
// 	fmt.Printf("Memory Used: %d bytes\n", memUsed)

// 	return nil // Success
// }

// Notify event (placeholder function)
func Notify(eventName string, content Index) error {
	fmt.Println(eventName, "Notified")
	return nil
}
