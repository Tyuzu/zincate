package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func InitializeHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	requester := payload["requester"]
	service := payload["service"]
	action := payload["action"]

	if requester == "register" {
		userID := payload["userid"].(string)
		initializeUserDefaults(userID)
	} else if service == "event" {
		userID := payload["userid"].(string)
		eventID := payload["eventid"].(string)

		if action == "create" {
			handleEventCreation(userID, eventID)
		} else if action == "delete" {
			handleEventDeletion(userID, eventID)
		}
	}
	w.WriteHeader(http.StatusOK)
}

// func InitializeHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	var payload map[string]interface{}
// 	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
// 		http.Error(w, "Invalid payload", http.StatusBadRequest)
// 		return
// 	}

// 	requester := payload["requester"]
// 	service := payload["service"]
// 	action := payload["action"]

// 	if requester == "register" {
// 		userID := payload["userid"].(string)
// 		initializeUserDefaults(userID)
// 	} else if service == "event" {
// 		userID := payload["userid"].(string)
// 		eventID := payload["eventid"].(string)

// 		if action == "create" {
// 			handleEventCreation(userID, eventID)
// 		} else if action == "delete" {
// 			handleEventDeletion(userID, eventID)
// 		}
// 	}
// 	w.WriteHeader(http.StatusOK)
// }

func initializeUserDefaults(userID string) {
	// Create default documents in profiles, settings, and user_data
	// Example: Insert default user profile into MongoDB
	profile := bson.M{
		"userid":   userID,
		"username": fmt.Sprintf("user_%s", userID),
		"avatar":   "default.png",
	}
	_, err := profilesCollection.InsertOne(context.Background(), profile)
	if err != nil {
		log.Printf("Error initializing user defaults: %v", err)
	}
	initializeSettings(userID)
}

func handleEventCreation(userID, eventID string) {
	// Update user_data collection to add the new event
	filter := bson.M{"userid": userID}
	update := bson.M{"$push": bson.M{"events": eventID}}
	_, err := userDataCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		log.Printf("Error adding event: %v", err)
	}
}

func handleEventDeletion(userID, eventID string) {
	// Update user_data collection to remove the event
	filter := bson.M{"userid": userID}
	update := bson.M{"$pull": bson.M{"events": eventID}}
	_, err := userDataCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		log.Printf("Error deleting event: %v", err)
	}

	// Clean up other collections (e.g., tickets, merchandise, media)
	deleteRelatedCollections(eventID)
}

func deleteRelatedCollections(eventID string) {
	// Example: Delete all tickets associated with the event
	filter := bson.M{"eventid": eventID}
	_, err := ticketsCollection.DeleteMany(context.Background(), filter)
	if err != nil {
		log.Printf("Error deleting related collections: %v", err)
	}
}

// Initialize default settings for a new user
func initializeSettings(userID string) error {
	defaultSettings := []Setting{
		{"theme", "Light", "User theme preference"},
		{"notifications", true, "Enable notifications"},
		{"privacy_mode", false, "Enable privacy mode"},
		{"auto_logout", true, "Enable auto logout after inactivity"},
		{"language", "English", "Preferred language"},
		{"time_zone", "UTC", "Time zone preference"},
		{"daily_reminder", "08:00", "Daily reminder time"},
	}

	userSettings := UserSettings{
		UserID:   userID,
		Settings: defaultSettings,
	}

	_, err := settingsCollection.InsertOne(context.TODO(), userSettings)
	return err
}
