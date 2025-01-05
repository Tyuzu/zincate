package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// Event struct (example, adjust fields as necessary)
type Eevent struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Location string `json:"location"`
	Price    int    `json:"price"` // Price in cents
}

// Search handler
func searchEvents(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	_ = ps.ByName("entityType") //entityType

	query := r.URL.Query()
	searchQuery := query.Get("query")
	category := query.Get("category")
	location := query.Get("location")
	maxPrice, err := strconv.Atoi(query.Get("maxPrice"))
	if err != nil {
		maxPrice = 1000 // Default max price
	}

	// Build search filter (this should be connected to your DB)
	filter := bson.M{}

	if searchQuery != "" {
		filter["name"] = bson.M{"$regex": searchQuery, "$options": "i"} // Search by name (case-insensitive)
	}
	if category != "" {
		filter["category"] = category
	}
	if location != "" {
		filter["location"] = location
	}
	if maxPrice > 0 {
		filter["price"] = bson.M{"$lte": maxPrice} // Filter by max price
	}

	// Query the database
	collection := client.Database("eventdb").Collection("events")
	cursor, err := collection.Find(context.TODO(), filter)
	if err != nil {
		http.Error(w, "Error fetching events", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	// Fetch results and send back as JSON
	// var events []Eevent
	// if err := cursor.All(context.TODO(), &events); err != nil {
	// 	http.Error(w, "Error decoding events", http.StatusInternalServerError)
	// 	return
	// }

	event1 := Eevent{
		ID:       "32432dfgf",
		Name:     "fefr",
		Category: "fewfre",
		Location: "rf",
		Price:    3,
	}
	event2 := Eevent{
		ID:       "32432dfgf",
		Name:     "fefr",
		Category: "fewfre",
		Location: "rf",
		Price:    3,
	}
	var events = []Eevent{event1, event2}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}
