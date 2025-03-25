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
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Redis client for Pub/Sub
var redisClient = redis.NewClient(&redis.Options{
	Addr: "localhost:6379", // Update if using a different Redis setup
})

// Log multiple activities and publish events to Redis
func LogActivities(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

	var activities []structs.Activity
	if err := json.NewDecoder(r.Body).Decode(&activities); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "Invalid input")
		log.Println("Failed to decode activities:", err)
		return
	}

	// Add metadata to each activity
	for i := range activities {
		activities[i].Username = claims.Username
		activities[i].Timestamp = time.Now()
	}

	// Insert all activities in one batch
	var docs []interface{}
	for _, activity := range activities {
		docs = append(docs, activity)
	}

	_, err = db.ActivitiesCollection.InsertMany(context.TODO(), docs)
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, "Failed to log activities")
		log.Println("Failed to insert activities into database:", err)
		return
	}

	// Publish each activity to Redis for real-time recommendations
	for _, activity := range activities {
		activityJSON, _ := json.Marshal(activity)
		err := redisClient.Publish(context.TODO(), "activity_events", activityJSON).Err()
		if err != nil {
			log.Println("Failed to publish activity to Redis:", err)
		}
	}

	log.Println("Activities logged and published to Redis:", activities)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"message": "Activities logged successfully"}`))
}

// Fetch user activity feed
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

// Fetch trending activities (latest across all users)
func GetTrendingActivities(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	opts := options.Find()
	opts.SetSort(bson.M{"timestamp": -1}) // Sort by newest first
	opts.SetLimit(10)                     // Fetch latest 10 activities

	cursor, err := db.ActivitiesCollection.Find(context.TODO(), bson.M{}, opts)
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, "Failed to fetch trending activities")
		return
	}
	defer cursor.Close(context.TODO())

	var activities []structs.Activity
	if err := cursor.All(context.TODO(), &activities); err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, "Failed to decode trending activities")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activities)
	log.Println("Fetched trending activities:", activities)
}

// Utility function for sending error responses
func SendErrorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// Redis subscriber for real-time recommendations
func SubscribeToActivityEvents() {
	pubsub := redisClient.Subscribe(context.TODO(), "activity_events")
	ch := pubsub.Channel()

	for msg := range ch {
		var activity structs.Activity
		if err := json.Unmarshal([]byte(msg.Payload), &activity); err != nil {
			log.Println("Failed to decode activity from Redis:", err)
			continue
		}

		log.Println("Received activity event from Redis:", activity)

		// Here, you can process the activity and update recommendations in real-time
		ProcessActivityForRecommendations(activity)
	}
}

// Process activity data and update recommendations
func ProcessActivityForRecommendations(activity structs.Activity) {
	// Example: Check if activity is a ticket purchase
	if activity.Action == "purchase" {
		log.Println("Processing purchase recommendation for:", activity.Username)
		// Add logic to recommend related events or products
	}

	// Example: If activity is an event creation, recommend it to interested users
	if activity.Action == "event_created" {
		log.Println("Processing event recommendation for:", activity.Username)
		// Add logic to push event to users with similar interests
	}

	// Extend this logic for other activity types
}
