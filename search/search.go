package search

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"naevis/models"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
)

// EventHandler processes incoming event requests (POST, PUT, DELETE).
func EventHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Allow only POST, PUT, and DELETE.
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
		http.Error(w, "Only POST, PUT, and DELETE requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var event models.Index
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "Invalid JSON for event", http.StatusBadRequest)
		return
	}
	log.Printf("Received event: %+v", event)

	IndexDatainRedis(event)

	log.Printf("Indexed")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, `{"message": "Event processed successfully"}`)
}

// Autocompleter handles HTTP requests for autocomplete suggestions.
func Autocompleter(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		http.Error(w, "Search prefix is required", http.StatusBadRequest)
		return
	}
	prefix = strings.ToLower(prefix)

	// Retrieve suggestions.
	results, err := GetWordsWithPrefix(prefix)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving autocomplete: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to JSON and respond.
	response, err := json.Marshal(results)
	if err != nil {
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func SearchHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entityType") // Extract active tab type
	log.Println("Received search request for:", entityType)

	query := r.URL.Query().Get("query")

	if query == "" {
		http.Error(w, "Search query is required", http.StatusBadRequest)
		return
	}
	query = strings.ToLower(query)
	GetResultsByTypeHandler(w, r, entityType, query)
}

/****************AUTOCOMPLETE with * and ? **********************/

func (t *TrieNode) SearchWithWildcard(query string, index int) bool {
	if index == len(query) {
		return t.IsWord
	}

	ch := rune(query[index])

	if ch == '*' || ch == '?' {
		for _, child := range t.Children {
			if child.SearchWithWildcard(query, index+1) {
				return true
			}
		}
		return false
	}

	if child, exists := t.Children[ch]; exists {
		return child.SearchWithWildcard(query, index+1)
	}

	return false
}
