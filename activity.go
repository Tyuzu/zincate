package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// // JWT claims
// type Claims struct {
// 	Username string `json:"username"`
// 	UserID   string `json:"userId"`
// 	jwt.RegisteredClaims
// }

// var (
// 	// tokenSigningAlgo = jwt.SigningMethodHS256
// 	jwtSecret = []byte("your_secret_key") // Replace with a secure secret key
// )

func logActivity(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	if len(tokenString) < 8 {
		sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
		log.Println("Authorization token is missing or invalid.")
		return
	}

	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Invalid token")
		log.Println("Invalid token:", err)
		return
	}

	var activity Activity
	if err := json.NewDecoder(r.Body).Decode(&activity); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid input")
		log.Println("Failed to decode activity:", err)
		return
	}

	activity.Username = claims.Username
	activity.Timestamp = time.Now()

	activitiesCollection := client.Database("eventdb").Collection("activities")
	_, err = activitiesCollection.InsertOne(context.TODO(), activity)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to log activity")
		log.Println("Failed to insert activity into database:", err)
		return
	}

	log.Println("Activity logged:", activity)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)                              // Respond with 201 Created
	w.Write([]byte(`{"message": "Activity logged successfully"}`)) // Include a response body
}

// Fetch activity feed
func getActivityFeed(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	if len(tokenString) < 8 {
		sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		sendErrorResponse(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	activitiesCollection := client.Database("eventdb").Collection("activities")
	cursor, err := activitiesCollection.Find(context.TODO(), bson.M{"username": claims.Username})
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to fetch activities")
		return
	}
	defer cursor.Close(context.TODO())

	var activities []Activity
	if err := cursor.All(context.TODO(), &activities); err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "Failed to decode activities")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activities)
	log.Println("Fetched activities:", activities)
}

func sendErrorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
