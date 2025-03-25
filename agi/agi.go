package agi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
)

// Sample data for personalization
var sampleData = map[string][]map[string]string{
	"recommended_events": {
		{"title": "🎉 Music Festival", "location": "City Park"},
		{"title": "📅 Tech Conference", "location": "Convention Center"},
		{"title": "🎭 Theatre Night", "location": "Grand Theatre"},
		{"title": "🎸 Rock Concert", "location": "Downtown Arena"},
		{"title": "💻 Hackathon", "location": "Tech Hub"},
		{"title": "📅 Tech Conference", "location": "Convention Center"},
		{"title": "🎭 Theatre Night", "location": "Grand Theatre"},
		{"title": "🎸 Rock Concert", "location": "Downtown Arena"},
		{"title": "💻 Hackathon", "location": "Tech Hub"},
	},
	"recommended_places": {
		{"name": "🌅 Sunset Cafe", "location": "Downtown"},
		{"name": "🏞️ River Walk", "location": "City Outskirts"},
		{"name": "☕ Coffee Corner", "location": "City Center"},
		{"name": "🍴 Diner Delight", "location": "Uptown"},
	},
	"followed_posts": {
		{"user": "Alice", "content": "Had an amazing time at the festival!"},
		{"user": "Bob", "content": "Tech Conference was insightful!"},
		{"user": "Charlie", "content": "The theatre show was mesmerizing."},
		{"user": "Alice", "content": "Rock Concert rocked my world!"},
	},
	"ads": {
		{"title": "🔥 Special Discount at Sunset Cafe!", "description": "Get 20% off your next visit."},
		{"title": "🎟️ Early Bird Offer!", "description": "Book your tickets now for the upcoming festival."},
	},
}

// Pagination helper function
func paginateList(data []map[string]string, page, itemsPerPage int) []map[string]string {
	start := (page - 1) * itemsPerPage
	end := start + itemsPerPage

	if start >= len(data) {
		return []map[string]string{} // Return empty if out of range
	}

	if end > len(data) {
		end = len(data)
	}
	return data[start:end]
}

// GetHomeFeed handles the home feed API
func GetHomeFeed(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	// Parse request body
	var requestData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// Extract user_id, section, and page
	userID, ok := requestData["user_id"].(string)
	if !ok || strings.TrimSpace(userID) == "" {
		http.Error(w, `{"error": "User ID is required"}`, http.StatusBadRequest)
		return
	}

	section, ok := requestData["section"].(string)
	if !ok || strings.TrimSpace(section) == "" {
		http.Error(w, `{"error": "Invalid section"}`, http.StatusBadRequest)
		return
	}

	page := 1 // Default page = 1
	if p, ok := requestData["page"].(float64); ok {
		page = int(p)
	}

	// Check if section exists
	data, exists := sampleData[section]
	if !exists {
		http.Error(w, `{"error": "Invalid section"}`, http.StatusBadRequest)
		return
	}

	// Paginate and return data
	itemsPerPage := 3
	response := paginateList(data, page, itemsPerPage)

	json.NewEncoder(w).Encode(response)
}
