package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// // Middleware for JWT Authentication (stub)
// func authMiddleware(next httprouter.Handle) httprouter.Handle {
// 	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 		claims, ok := r.Context().Value(userIDKey).(*Claims) // Assume Claims has been validated elsewhere
// 		if !ok {
// 			http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 			return
// 		}
// 		userID := claims.UserID
// 		ctx := context.WithValue(r.Context(), userIDKey, userID)
// 		next(w, r.WithContext(ctx), ps)
// 	}
// }

// // Initialize default settings for a new user
// func initializeSettings(userID string) error {

// 	defaultSettings := []Setting{
// 		{"theme", "Light", "User theme preference"},
// 		{"notifications", true, "Enable notifications"},
// 		{"privacy_mode", false, "Enable privacy mode"},
// 		{"auto_logout", true, "Enable auto logout after inactivity"},
// 		{"language", "English", "Preferred language"},
// 		{"time_zone", "UTC", "Time zone preference"},
// 		{"daily_reminder", "08:00", "Daily reminder time"},
// 	}

// 	userSettings := UserSettings{
// 		UserID:   userID,
// 		Settings: defaultSettings,
// 	}

// 	_, err := settingsCollection.InsertOne(context.TODO(), userSettings)
// 	return err
// }

// Get user settings
func initUserSettings(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID := r.Context().Value(userIDKey).(string)
	if ps.ByName("userid") == userID {
		initializeSettings(userID)
		a, _ := json.Marshal(true)
		fmt.Fprint(w, string(a))
	}
	a, _ := json.Marshal(false)
	fmt.Fprint(w, string(a))
}

// Get user settings
func getUserSettings(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID := r.Context().Value(userIDKey).(string)

	var userSettings UserSettings
	err := settingsCollection.FindOne(context.TODO(), bson.M{"userID": userID}).Decode(&userSettings)
	if err == mongo.ErrNoDocuments {
		http.Error(w, "Settings not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userSettings)
}

// Update a specific user setting
func updateUserSetting(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID := r.Context().Value(userIDKey).(string)
	var update struct {
		Type  string      `json:"type"`
		Value interface{} `json:"value"`
	}

	err := json.NewDecoder(r.Body).Decode(&update)
	if err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	filter := bson.M{"userID": userID, "settings.type": update.Type}
	updateDoc := bson.M{"$set": bson.M{"settings.$.value": update.Value}}

	result, err := settingsCollection.UpdateOne(context.TODO(), filter, updateDoc)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if result.MatchedCount == 0 {
		http.Error(w, "Setting not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Setting updated successfully")
}
