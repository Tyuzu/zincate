package search

import (
	"fmt"
	"log"
	"naevis/mq"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Generic entity struct
type SearchResult struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Date     string `json:"date,omitempty"`
	Category string `json:"category,omitempty"`
	Location string `json:"location,omitempty"`
	Price    int    `json:"price,omitempty"`
}

// Search handler (fetches based on active tab)
func SearchHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entityType") // Extract active tab type
	log.Println("Received search request for:", entityType)

	query := r.URL.Query().Get("query")

	if query == "" {
		http.Error(w, "Search query is required", http.StatusBadRequest)
		return
	}

	var results, err = mq.FetchResults(entityType, query)
	if err != nil {
		log.Println(err)
	}

	// fmt.Println(results)
	fmt.Fprintf(w, "%s", string(results))
}
