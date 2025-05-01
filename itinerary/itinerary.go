// itinerary.go
package itinerary

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"naevis/db"
	"naevis/profile"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// Itinerary represents the travel itinerary
type Itinerary struct {
	ItineraryID string  `json:"itineraryid" bson:"itineraryid,omitempty"`
	UserID      string  `json:"user_id" bson:"user_id"`
	Name        string  `json:"name" bson:"name"`
	Description string  `json:"description" bson:"description"`
	StartDate   string  `json:"start_date" bson:"start_date"`
	EndDate     string  `json:"end_date" bson:"end_date"`
	Status      string  `json:"status" bson:"status"` // Draft/Confirmed
	Published   bool    `json:"published" bson:"published"`
	ForkedFrom  *string `json:"forked_from,omitempty" bson:"forked_from,omitempty"`
	Deleted     bool    `json:"-" bson:"deleted,omitempty"` // Internal use only
	// the new day-by-day schedule
	Days []Day `json:"days" bson:"days"`
}

// add these at the top, just below package declaration
type Visit struct {
	Location  string `json:"location" bson:"location"`
	StartTime string `json:"start_time" bson:"start_time"`
	EndTime   string `json:"end_time" bson:"end_time"`
	// nil for the very first visit of a day
	Transport *string `json:"transport,omitempty" bson:"transport,omitempty"`
}

type Day struct {
	Date   string  `json:"date" bson:"date"`
	Visits []Visit `json:"visits" bson:"visits"`
}

// Utility function to extract user ID from JWT
func GetRequestingUserID(w http.ResponseWriter, r *http.Request) string {
	tokenString := r.Header.Get("Authorization")
	claims, err := profile.ValidateJWT(tokenString)
	if err != nil {
		log.Printf("JWT validation error: %v", err)
		return ""
	}
	return claims.UserID
}

// GET /api/itineraries
func GetItineraries(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"deleted": bson.M{"$ne": true}}

	cursor, err := db.ItineraryCollection.Find(ctx, filter)
	if err != nil {
		http.Error(w, "Error fetching itineraries", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var itineraries []Itinerary
	for cursor.Next(ctx) {
		var itinerary Itinerary
		if err := cursor.Decode(&itinerary); err == nil {
			itineraries = append(itineraries, itinerary)
		}
	}

	if itineraries == nil {
		itineraries = []Itinerary{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(itineraries)
}

// POST /api/itineraries
func CreateItinerary(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var itinerary Itinerary
	if err := json.NewDecoder(r.Body).Decode(&itinerary); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	userID := GetRequestingUserID(w, r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	itinerary.UserID = userID
	itinerary.ItineraryID = utils.GenerateID(13)
	if itinerary.Status == "" {
		itinerary.Status = "Draft"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ItineraryCollection.InsertOne(ctx, itinerary)
	if err != nil {
		http.Error(w, "Error inserting itinerary", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

// GET /api/itineraries/all/:id
func GetItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	itineraryID := ps.ByName("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"itineraryid": itineraryID, "deleted": bson.M{"$ne": true}}

	var itinerary Itinerary
	err := db.ItineraryCollection.FindOne(ctx, filter).Decode(&itinerary)
	if err != nil {
		http.Error(w, "Itinerary not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(itinerary)
}

// PUT /api/itineraries/:id
func UpdateItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID := GetRequestingUserID(w, r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	itineraryID := ps.ByName("id")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var existing Itinerary
	err := db.ItineraryCollection.FindOne(ctx, bson.M{"itineraryid": itineraryID}).Decode(&existing)
	if err != nil {
		http.Error(w, "Itinerary not found", http.StatusNotFound)
		return
	}

	if existing.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var updated Itinerary
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	update := bson.M{"$set": bson.M{
		"name":        updated.Name,
		"description": updated.Description,
		"start_date":  updated.StartDate,
		"end_date":    updated.EndDate,
		"status":      updated.Status,
		"published":   updated.Published,
		"days":        updated.Days,
	}}

	_, err = db.ItineraryCollection.UpdateOne(ctx, bson.M{"itineraryid": itineraryID}, update)
	if err != nil {
		http.Error(w, "Error updating itinerary", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(bson.M{"message": "Itinerary updated successfully"})
}

// DELETE /api/itineraries/:id
func DeleteItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID := GetRequestingUserID(w, r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	itineraryID := ps.ByName("id")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var itinerary Itinerary
	err := db.ItineraryCollection.FindOne(ctx, bson.M{"itineraryid": itineraryID}).Decode(&itinerary)
	if err != nil {
		http.Error(w, "Itinerary not found", http.StatusNotFound)
		return
	}

	if itinerary.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	update := bson.M{"$set": bson.M{"deleted": true}}
	_, err = db.ItineraryCollection.UpdateOne(ctx, bson.M{"itineraryid": itineraryID}, update)
	if err != nil {
		http.Error(w, "Error deleting itinerary", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(bson.M{"message": "Itinerary deleted successfully"})
}

// POST /api/itineraries/:id/fork
func ForkItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID := GetRequestingUserID(w, r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	originalID := ps.ByName("id")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var original Itinerary
	err := db.ItineraryCollection.FindOne(ctx, bson.M{"itineraryid": originalID}).Decode(&original)
	if err != nil {
		http.Error(w, "Original itinerary not found", http.StatusNotFound)
		return
	}

	newItinerary := Itinerary{
		ItineraryID: utils.GenerateID(13),
		UserID:      userID,
		Name:        "Forked - " + original.Name,
		Description: original.Description,
		StartDate:   original.StartDate,
		EndDate:     original.EndDate,
		Days:        original.Days,
		Status:      "Draft",
		Published:   false,
		ForkedFrom:  &originalID,
	}

	result, err := db.ItineraryCollection.InsertOne(ctx, newItinerary)
	if err != nil {
		http.Error(w, "Error forking itinerary", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

// PUT /api/itineraries/:id/publish
func PublishItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID := GetRequestingUserID(w, r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id := ps.ByName("id")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"itineraryid": id, "user_id": userID}
	update := bson.M{"$set": bson.M{"published": true}}

	result, err := db.ItineraryCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		http.Error(w, "Error publishing itinerary", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// GET /api/itineraries/search
func SearchItineraries(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	query := r.URL.Query()

	filter := bson.M{"deleted": bson.M{"$ne": true}}
	if start := query.Get("start_date"); start != "" {
		filter["start_date"] = start
	}
	if location := query.Get("location"); location != "" {
		// filter["locations"] = bson.M{"$in": []string{location}}
		filter["days.visits.location"] = bson.M{"$in": []string{location}}
	}
	if status := query.Get("status"); status != "" {
		filter["status"] = status
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := db.ItineraryCollection.Find(ctx, filter)
	if err != nil {
		http.Error(w, "Error fetching itineraries", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var itineraries []Itinerary
	for cursor.Next(ctx) {
		var itinerary Itinerary
		if err := cursor.Decode(&itinerary); err == nil {
			itineraries = append(itineraries, itinerary)
		}
	}

	if itineraries == nil {
		itineraries = []Itinerary{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(itineraries)
}
