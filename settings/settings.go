package settings

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/globals"
	"naevis/mq"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// UserSettings represents user settings
type UserSettings struct {
	UserID        string `json:"userID,omitempty" bson:"userID"`
	Theme         string `json:"theme" bson:"theme"`
	Notifications bool   `json:"notifications" bson:"notifications"`
	PrivacyMode   bool   `json:"privacy_mode" bson:"privacy_mode"`
	AutoLogout    bool   `json:"auto_logout" bson:"auto_logout"`
	Language      string `json:"language" bson:"language"`
	TimeZone      string `json:"time_zone" bson:"time_zone"`
	DailyReminder string `json:"daily_reminder" bson:"daily_reminder"`
}

// Default settings if user settings don't exist
func getDefaultSettings(userID string) UserSettings {
	return UserSettings{
		UserID:        userID,
		Theme:         "light",
		Notifications: true,
		PrivacyMode:   false,
		AutoLogout:    false,
		Language:      "english",
		TimeZone:      "UTC",
		DailyReminder: "09:00",
	}
}

// Fetch user settings as an array (frontend expects this format)
func GetUserSettings(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID := r.Context().Value(globals.UserIDKey).(string)

	var userSettings UserSettings
	err := db.SettingsCollection.FindOne(context.TODO(), bson.M{"userID": userID}).Decode(&userSettings)
	if err == mongo.ErrNoDocuments {
		// Initialize settings if missing
		userSettings = getDefaultSettings(userID)
		_, _ = db.SettingsCollection.InsertOne(context.TODO(), userSettings)
	} else if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Convert to array format expected by frontend
	settingsArray := []map[string]any{
		{"type": "theme", "value": userSettings.Theme, "description": "Choose theme mode"},
		{"type": "notifications", "value": userSettings.Notifications, "description": "Enable notifications"},
		{"type": "privacy_mode", "value": userSettings.PrivacyMode, "description": "Enable privacy mode"},
		{"type": "auto_logout", "value": userSettings.AutoLogout, "description": "Enable auto logout"},
		{"type": "language", "value": userSettings.Language, "description": "Select language"},
		{"type": "time_zone", "value": userSettings.TimeZone, "description": "Select time zone"},
		{"type": "daily_reminder", "value": userSettings.DailyReminder, "description": "Set daily reminder"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settingsArray)
}

// Update a specific user setting
func UpdateUserSetting(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID := r.Context().Value(globals.UserIDKey).(string)
	settingType := ps.ByName("type")

	validSettings := map[string]bool{
		"theme":          true,
		"notifications":  true,
		"privacy_mode":   true,
		"auto_logout":    true,
		"language":       true,
		"time_zone":      true,
		"daily_reminder": true,
	}
	if !validSettings[settingType] {
		http.Error(w, "Invalid setting type", http.StatusBadRequest)
		return
	}

	var update struct {
		Value any `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	// Update MongoDB document
	filter := bson.M{"userID": userID}
	updateDoc := bson.M{"$set": bson.M{settingType: update.Value}}

	opts := options.Update().SetUpsert(true)
	_, err := db.SettingsCollection.UpdateOne(context.TODO(), filter, updateDoc, opts)
	if err != nil {
		http.Error(w, "Failed to update setting", http.StatusInternalServerError)
		return
	}

	m := mq.Index{}
	mq.Notify("settings-updated", m)

	// Respond with proper JSON
	response := map[string]any{
		"status":  "success",
		"message": "Setting updated successfully",
		"type":    settingType,
		"value":   update.Value,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Initialize user settings if they don't exist
func InitUserSettings(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID := r.Context().Value(globals.UserIDKey).(string)

	// Check if user settings already exist
	var existingSettings UserSettings
	err := db.SettingsCollection.FindOne(context.TODO(), bson.M{"userID": userID}).Decode(&existingSettings)
	if err == mongo.ErrNoDocuments {
		// No settings found, create default settings
		newSettings := getDefaultSettings(userID)
		_, err := db.SettingsCollection.InsertOne(context.TODO(), newSettings)
		if err != nil {
			http.Error(w, "Failed to initialize settings", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(true) // Successfully initialized
		return
	} else if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// If settings already exist, return false
	json.NewEncoder(w).Encode(false)
}
