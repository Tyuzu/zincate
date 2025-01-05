package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func suggestFollowers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	claims, ok := r.Context().Value(userIDKey).(*Claims)
	if !ok || claims.UserID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Pagination parameters
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 {
		limit = 10
	}

	skip := (page - 1) * limit

	// Fetch the current user's follows from the followings collection
	var followData UserFollow
	err = followingsCollection.FindOne(context.TODO(), bson.M{"userid": claims.UserID}).Decode(&followData)
	if err != nil && err != mongo.ErrNoDocuments {
		http.Error(w, "Failed to fetch follow data", http.StatusInternalServerError)
		return
	}

	// Construct the exclusion filter for already followed users and the current user
	excludedUserIDs := append(followData.Follows, claims.UserID)

	// Query for suggested users excluding the current user and already followed users
	filter := bson.M{"userid": bson.M{"$nin": excludedUserIDs}}
	options := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := followingsCollection.Find(context.TODO(), filter, options)
	if err != nil {
		http.Error(w, "Failed to fetch suggestions", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	// Collect suggested users
	suggestedUsers := []UserSuggest{}
	for cursor.Next(context.TODO()) {
		var suggestedUser UserSuggest
		if err := cursor.Decode(&suggestedUser); err == nil {
			suggestedUsers = append(suggestedUsers, suggestedUser)
		}
	}

	// Return the suggested users or an empty list if none are found
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggestedUsers)
}

/***************************************************/
func getPlaceSuggestions(ctx context.Context, query string) ([]Suggestion, error) {
	var suggestions []Suggestion

	// Use Redis KEYS command to find matching place suggestions by name
	// (this is a simple approach, you may want a more efficient search strategy)
	keys, err := conn.Keys(ctx, fmt.Sprintf("suggestions:place:%s*", query)).Result()
	if err != nil {
		return nil, err
	}

	// Retrieve the corresponding place data
	for _, key := range keys {
		var suggestion Suggestion
		err := conn.Get(ctx, key).Scan(&suggestion)
		if err != nil {
			return nil, err
		}
		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
}

func suggestionsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	suggestions, err := getPlaceSuggestions(ctx, query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching suggestions: %v", err), http.StatusInternalServerError)
		return
	}
	log.Println("handler sugg : ", suggestions)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(suggestions)
}
