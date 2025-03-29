package itinerary

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/db"
	"naevis/profile"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func GetrequestingUserID(w http.ResponseWriter, r *http.Request) string {
	// // Retrieve user ID
	// requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	// if !ok {
	// 	return ""
	// }
	// return requestingUserID

	tokenString := r.Header.Get("Authorization")
	claims, err := profile.ValidateJWT(tokenString)
	if err != nil {
		log.Printf("JWT validation error: %v", err)
		// http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return ""
	}
	return claims.UserID
}

type Itinerary struct {
	ItineraryID string   `json:"itineraryid" bson:"itineraryid,omitempty"`
	UserID      string   `json:"user_id" bson:"user_id"`
	Name        string   `json:"name" bson:"name"`
	Description string   `json:"description" bson:"description"`
	StartDate   string   `json:"start_date" bson:"start_date"`
	EndDate     string   `json:"end_date" bson:"end_date"`
	Locations   []string `json:"locations" bson:"locations"`
	Status      string   `json:"status" bson:"status"`                               // "Draft" or "Confirmed"
	Published   bool     `json:"published" bson:"published"`                         // true = visible to others
	ForkedFrom  *string  `json:"forked_from,omitempty" bson:"forked_from,omitempty"` // Original itinerary ID
}

func GetItineraries(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"deleted": bson.M{"$ne": true}} // Only fetch non-deleted itineraries

	var itineraries []Itinerary
	cursor, err := db.IternaryCollection.Find(ctx, filter)
	if err != nil {
		http.Error(w, "Error fetching itineraries", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var itinerary Itinerary
		cursor.Decode(&itinerary)
		itineraries = append(itineraries, itinerary)
	}

	if itineraries == nil {
		itineraries = []Itinerary{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(itineraries)
}

func CreateItinerary(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var itinerary Itinerary
	json.NewDecoder(r.Body).Decode(&itinerary)

	if itinerary.Status == "" {
		itinerary.Status = "Draft" // Default to draft if not provided
	}

	itinerary.UserID = GetrequestingUserID(w, r)
	itinerary.ItineraryID = utils.GenerateID(13)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.IternaryCollection.InsertOne(ctx, itinerary)
	if err != nil {
		http.Error(w, "Error inserting itinerary", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

func PublishItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	itineraryID := ps.ByName("id")
	filter := bson.M{"itineraryid": itineraryID}
	update := bson.M{"$set": bson.M{"published": true}}

	// GetrequestingUserID(w, r)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.IternaryCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		http.Error(w, "Error publishing itinerary", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func ForkItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	originalID := ps.ByName("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var originalItinerary Itinerary
	err := db.IternaryCollection.FindOne(ctx, bson.M{"itineraryid": originalID}).Decode(&originalItinerary)
	if err != nil {
		http.Error(w, "Original itinerary not found", http.StatusNotFound)
		return
	}
	// GetrequestingUserID(w, r)

	// Create a new itinerary with a new ID but reference the original
	newItinerary := Itinerary{
		ItineraryID: utils.GenerateID(13),
		UserID:      GetrequestingUserID(w, r), // Replace with logged-in user ID
		Name:        "Forked - " + originalItinerary.Name,
		Description: originalItinerary.Description,
		StartDate:   originalItinerary.StartDate,
		EndDate:     originalItinerary.EndDate,
		Locations:   originalItinerary.Locations,
		Status:      "Draft",
		Published:   false, // Forked itineraries start as private drafts
		ForkedFrom:  &originalID,
	}

	result, err := db.IternaryCollection.InsertOne(ctx, newItinerary)
	if err != nil {
		http.Error(w, "Error forking itinerary", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

func SearchItineraries(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	queryParams := r.URL.Query()

	filter := bson.M{}
	if startDate := queryParams.Get("start_date"); startDate != "" {
		filter["start_date"] = startDate
	}
	if location := queryParams.Get("location"); location != "" {
		filter["locations"] = bson.M{"$in": []string{location}}
	}
	if status := queryParams.Get("status"); status != "" {
		filter["status"] = status
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var itineraries []Itinerary
	cursor, err := db.IternaryCollection.Find(ctx, filter)
	if err != nil {
		http.Error(w, "Error fetching itineraries", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var itinerary Itinerary
		cursor.Decode(&itinerary)
		itineraries = append(itineraries, itinerary)
	}

	if itineraries == nil {
		itineraries = []Itinerary{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(itineraries)
}

func GetItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	itineraryID := ps.ByName("id")
	fmt.Println(itineraryID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// filter := bson.M{"deleted": bson.M{"$ne": true, "itineraryid": itineraryID}} // Only fetch non-deleted itineraries
	filter := bson.M{"itineraryid": itineraryID}

	var itinerary Itinerary
	err := db.IternaryCollection.FindOne(ctx, filter).Decode(&itinerary)
	if err != nil {
		http.Error(w, "Itinerary not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(itinerary)
}

func UpdateItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// tokenString := r.Header.Get("Authorization")
	// claims, err := profile.ValidateJWT(tokenString)
	// if err != nil {
	// 	log.Printf("JWT validation error: %v", err)
	// 	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	// 	return
	// }
	// userID := claims.UserID

	userID := GetrequestingUserID(w, r)

	itineraryID := ps.ByName("id")

	// Fetch the existing itinerary to check ownership
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var itinerary Itinerary
	err := db.IternaryCollection.FindOne(ctx, bson.M{"itineraryid": itineraryID}).Decode(&itinerary)
	if err != nil {
		http.Error(w, "Itinerary not found", http.StatusNotFound)
		return
	}

	// Check if the user is the creator
	if itinerary.UserID != userID {
		http.Error(w, "Forbidden: You are not the owner of this itinerary", http.StatusForbidden)
		return
	}

	// Decode the updated data
	var updatedData Itinerary
	err = json.NewDecoder(r.Body).Decode(&updatedData)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update fields
	update := bson.M{"$set": bson.M{
		"name":        updatedData.Name,
		"description": updatedData.Description,
		"start_date":  updatedData.StartDate,
		"end_date":    updatedData.EndDate,
		"locations":   updatedData.Locations,
		"status":      updatedData.Status,
		"published":   updatedData.Published,
	}}

	_, err = db.IternaryCollection.UpdateOne(ctx, bson.M{"itineraryid": itineraryID}, update)
	if err != nil {
		http.Error(w, "Error updating itinerary", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(bson.M{"message": "Itinerary updated successfully"})
}

func DeleteItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// tokenString := r.Header.Get("Authorization")
	// claims, err := profile.ValidateJWT(tokenString)
	// if err != nil {
	// 	log.Printf("JWT validation error: %v", err)
	// 	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	// 	return
	// }
	// userID := claims.UserID
	userID := GetrequestingUserID(w, r)

	itineraryID := ps.ByName("id")

	// Fetch the existing itinerary to check ownership
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var itinerary Itinerary
	err := db.IternaryCollection.FindOne(ctx, bson.M{"itineraryid": itineraryID}).Decode(&itinerary)
	if err != nil {
		http.Error(w, "Itinerary not found", http.StatusNotFound)
		return
	}

	// Check if the user is the creator
	if itinerary.UserID != userID {
		http.Error(w, "Forbidden: You are not the owner of this itinerary", http.StatusForbidden)
		return
	}

	// // Delete itinerary
	// _, err = db.IternaryCollection.DeleteOne(ctx, bson.M{"itineraryid": itineraryID})
	// if err != nil {
	// 	http.Error(w, "Error deleting itinerary", http.StatusInternalServerError)
	// 	return
	// }

	//soft-delete
	update := bson.M{"$set": bson.M{"deleted": true}}
	_, err = db.IternaryCollection.UpdateOne(ctx, bson.M{"itineraryid": itineraryID}, update)
	if err != nil {
		http.Error(w, "Error deleting itinerary", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(bson.M{"message": "Itinerary deleted successfully"})
}
