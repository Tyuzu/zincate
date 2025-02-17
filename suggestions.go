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

// func suggestFollowers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	userID := r.Context().Value(userIDKey).(string)

// 	// Pagination parameters
// 	page, err := strconv.Atoi(r.URL.Query().Get("page"))
// 	if err != nil || page < 1 {
// 		page = 1
// 	}

// 	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
// 	if err != nil || limit < 1 {
// 		limit = 10
// 	}

// 	skip := (page - 1) * limit

// 	// Fetch the current user's follows from the user collection
// 	var followData UserFollow
// 	err = userCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&followData)
// 	if err != nil && err != mongo.ErrNoDocuments {
// 		http.Error(w, "Failed to fetch follow data", http.StatusInternalServerError)
// 		return
// 	}

// 	// Construct the exclusion filter for already followed users and the current user
// 	excludedUserIDs := append(followData.Follows, userID)

// 	// Query for suggested users excluding the current user and already followed users
// 	filter := bson.M{"userid": bson.M{"$nin": excludedUserIDs}}
// 	options := options.Find().
// 		SetSkip(int64(skip)).
// 		SetLimit(int64(limit))

// 	cursor, err := userCollection.Find(context.TODO(), filter, options)
// 	if err != nil {
// 		http.Error(w, "Failed to fetch suggestions", http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

//		// Collect suggested users
//		suggestedUsers := []UserSuggest{}
//		for cursor.Next(context.TODO()) {
//			var suggestedUser UserSuggest
//			if err := cursor.Decode(&suggestedUser); err == nil {
//				suggestedUsers = append(suggestedUsers, suggestedUser)
//			}
//		}
//		fmt.Println("suggestedUsers ::::::::: ", suggestedUsers)
//		// Return the suggested users or an empty list if none are found
//		w.Header().Set("Content-Type", "application/json")
//		json.NewEncoder(w).Encode(suggestedUsers)
//	}

func suggestFollowers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	currentUserID := r.URL.Query().Get("userid")
	if currentUserID == "" {
		http.Error(w, "Missing userid", http.StatusBadRequest)
		return
	}

	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok {
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

	// Fetch user's follow data
	var followData UserFollow
	err = followingsCollection.FindOne(context.TODO(), bson.M{"userid": currentUserID}).Decode(&followData)
	if err != nil && err != mongo.ErrNoDocuments {
		http.Error(w, "Failed to fetch follow data", http.StatusInternalServerError)
		return
	}

	// Exclude already followed users + current user
	excludedUserIDs := append(followData.Follows, currentUserID, userID)

	// Query for suggested users
	filter := bson.M{"userid": bson.M{"$nin": excludedUserIDs}}
	options := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetProjection(bson.M{
			"userid":   1,
			"username": 1,
			"bio":      1,
		})

	cursor, err := userCollection.Find(context.TODO(), filter, options)
	if err != nil {
		http.Error(w, "Failed to fetch suggestions", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	// Collect suggested users
	var suggestedUsers []UserSuggest
	for cursor.Next(context.TODO()) {
		var suggestedUser UserSuggest
		if err := cursor.Decode(&suggestedUser); err == nil {
			// Explicitly set is_following: false
			suggestedUser.IsFollowing = false
			suggestedUsers = append(suggestedUsers, suggestedUser)
		}
	}

	// Handle empty response case
	if len(suggestedUsers) == 0 {
		suggestedUsers = []UserSuggest{}
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(suggestedUsers); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// func suggestFollowers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	currentUserID := r.URL.Query().Get("userid")
// 	fmt.Println("curr us", currentUserID)
// 	userID, ok := r.Context().Value(userIDKey).(string)
// 	if !ok {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	// Pagination parameters
// 	page, err := strconv.Atoi(r.URL.Query().Get("page"))
// 	if err != nil || page < 1 {
// 		page = 1
// 	}

// 	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
// 	if err != nil || limit < 1 {
// 		limit = 10
// 	}

// 	skip := (page - 1) * limit

// 	// Fetch the list of users the current user follows
// 	var followData UserFollow
// 	err = followingsCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&followData)
// 	if err != nil && err != mongo.ErrNoDocuments {
// 		http.Error(w, "Failed to fetch follow data", http.StatusInternalServerError)
// 		return
// 	}

// 	fmt.Println("::--::--::--::")
// 	// Exclude already followed users + current user
// 	excludedUserIDs := append(followData.Follows, userID, currentUserID)
// 	// excludedUserIDs := []string{userID}

// 	// Query for suggested users excluding already followed ones
// 	filter := bson.M{"userid": bson.M{"$nin": excludedUserIDs}}
// 	options := options.Find().
// 		SetSkip(int64(skip)).
// 		SetLimit(int64(limit)).
// 		SetProjection(bson.M{
// 			"userid":   1,
// 			"username": 1,
// 			"bio":      1,
// 		})

// 	cursor, err := userCollection.Find(context.TODO(), filter, options)
// 	if err != nil {
// 		http.Error(w, "Failed to fetch suggestions", http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

// 	// Collect suggested users
// 	var suggestedUsers []UserSuggest
// 	for cursor.Next(context.TODO()) {
// 		var suggestedUser UserSuggest
// 		if err := cursor.Decode(&suggestedUser); err == nil {
// 			// Add `is_following: false` explicitly
// 			suggestedUsers = append(suggestedUsers, UserSuggest{
// 				UserID:   suggestedUser.UserID,
// 				Username: suggestedUser.Username,
// 				Bio:      suggestedUser.Bio,
// 			})
// 		}
// 	}

// 	fmt.Println("::--::--::--::")

// 	// createFollowEntry(userID)

// 	if len(suggestedUsers) == 0 {
// 		suggestedUsers = []UserSuggest{}
// 	}
// 	// Set response headers and send JSON response
// 	w.Header().Set("Content-Type", "application/json")
// 	if err := json.NewEncoder(w).Encode(suggestedUsers); err != nil {
// 		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// 	}
// }

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
	if len(suggestions) == 0 {
		suggestions = []Suggestion{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(suggestions)
}
