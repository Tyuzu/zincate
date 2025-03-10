package activity

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/middleware"
	"naevis/structs"
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

func LogActivity(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	if len(tokenString) < 8 {
		SendErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
		log.Println("Authorization token is missing or invalid.")
		return
	}

	claims := &middleware.Claims{}
	_, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (any, error) {
		return globals.JwtSecret, nil
	})
	if err != nil {
		SendErrorResponse(w, http.StatusUnauthorized, "Invalid token")
		log.Println("Invalid token:", err)
		return
	}

	var activity structs.Activity
	if err := json.NewDecoder(r.Body).Decode(&activity); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "Invalid input")
		log.Println("Failed to decode activity:", err)
		return
	}

	activity.Username = claims.Username
	activity.Timestamp = time.Now()

	// db.ActivitiesCollection := client.Database("eventdb").Collection("activities")
	_, err = db.ActivitiesCollection.InsertOne(context.TODO(), activity)
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, "Failed to log activity")
		log.Println("Failed to insert activity into database:", err)
		return
	}

	log.Println("Activity logged:", activity)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)                              // Respond with 201 Created
	w.Write([]byte(`{"message": "Activity logged successfully"}`)) // Include a response body
}

// Fetch activity feed
func GetActivityFeed(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	if len(tokenString) < 8 {
		SendErrorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	claims := &middleware.Claims{}
	_, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (any, error) {
		return globals.JwtSecret, nil
	})
	if err != nil {
		SendErrorResponse(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// db.ActivitiesCollection := client.Database("eventdb").Collection("activities")
	cursor, err := db.ActivitiesCollection.Find(context.TODO(), bson.M{"username": claims.Username})
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, "Failed to fetch activities")
		return
	}
	defer cursor.Close(context.TODO())

	var activities []structs.Activity
	if err := cursor.All(context.TODO(), &activities); err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, "Failed to decode activities")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activities)
	log.Println("Fetched activities:", activities)
}

func SendErrorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
