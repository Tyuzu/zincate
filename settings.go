package main

import (
	"context"
	"encoding/json"
	"fmt"
	"naevis/mq"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

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

	mq.Emit("settings-updated")

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Setting updated successfully")
}
