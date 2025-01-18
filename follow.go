package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/mq"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func doesFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	claims, ok := r.Context().Value(userIDKey).(*Claims)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := claims.UserID
	followedUserID := ps.ByName("id")

	if followedUserID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	var userFollow UserFollow
	err := followingsCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&userFollow)
	if err != nil && err != mongo.ErrNoDocuments {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	isFollowing := contains(userFollow.Follows, followedUserID)

	response := map[string]bool{"isFollowing": isFollowing}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getFollowers(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	claims, ok := r.Context().Value(userIDKey).(*Claims)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := claims.UserID

	var userFollow UserFollow
	err := followingsCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&userFollow)
	if err != nil && err != mongo.ErrNoDocuments {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	followers := []User{}
	for _, followerID := range userFollow.Followers {
		var follower User
		if err := userCollection.FindOne(context.TODO(), bson.M{"userid": followerID}).Decode(&follower); err == nil {
			followers = append(followers, follower)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(followers)
}

func getFollowing(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	claims, ok := r.Context().Value(userIDKey).(*Claims)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := claims.UserID

	var userFollow UserFollow
	err := followingsCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&userFollow)
	if err != nil && err != mongo.ErrNoDocuments {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	following := []User{}
	for _, followingID := range userFollow.Follows {
		var followUser User
		if err := userCollection.FindOne(context.TODO(), bson.M{"userid": followingID}).Decode(&followUser); err == nil {
			following = append(following, followUser)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(following)
}

func updateFollowRelationship(currentUserID, targetUserID, action string) error {
	if action != "follow" && action != "unfollow" {
		return fmt.Errorf("invalid action: %s", action)
	}

	// Update current user's follow list
	currentUserUpdate := bson.M{
		"$addToSet": bson.M{"follows": targetUserID},
	}
	if action == "unfollow" {
		currentUserUpdate = bson.M{
			"$pull": bson.M{"follows": targetUserID},
		}
	}
	_, err := followingsCollection.UpdateOne(
		context.TODO(),
		bson.M{"userid": currentUserID},
		currentUserUpdate,
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("failed to update current user's follows: %w", err)
	}

	// Update target user's followers list
	targetUserUpdate := bson.M{
		"$addToSet": bson.M{"followers": currentUserID},
	}
	if action == "unfollow" {
		targetUserUpdate = bson.M{
			"$pull": bson.M{"followers": currentUserID},
		}
	}
	_, err = followingsCollection.UpdateOne(
		context.TODO(),
		bson.M{"userid": targetUserID},
		targetUserUpdate,
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("failed to update target user's followers: %w", err)
	}

	mq.Emit("followed/unfllowed")

	return nil
}

func handleFollowAction(w http.ResponseWriter, r *http.Request, ps httprouter.Params, action string) {
	claims, ok := r.Context().Value(userIDKey).(*Claims)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	currentUserID := claims.UserID
	targetUserID := ps.ByName("id")

	if err := updateFollowRelationship(currentUserID, targetUserID, action); err != nil {
		log.Printf("Error updating follow relationship: %v", err)
		http.Error(w, "Failed to update follow relationship", http.StatusInternalServerError)
		return
	}

	response := map[string]any{
		"isFollowing": action == "follow",
		"ok":          true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func toggleFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handleFollowAction(w, r, ps, "follow")
}

func toggleUnFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	handleFollowAction(w, r, ps, "unfollow")
}

// func doesFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	claims, ok := r.Context().Value(userIDKey).(*Claims)
// 	if !ok {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}
// 	userID := claims.UserID

// 	followedUserId := ps.ByName("id")
// 	if followedUserId == "" {
// 		http.Error(w, "User ID is required", http.StatusBadRequest)
// 		return
// 	}

// 	log.Printf("User %s is trying to toggle follow for user %s", userID, followedUserId)

// 	// Retrieve the current user
// 	var currentUser User
// 	err := userCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&currentUser)
// 	if err != nil {
// 		http.Error(w, "User not found", http.StatusNotFound)
// 		return
// 	}

// 	// Check if the user is already following the followed user
// 	isFollowing := false
// 	for _, followedID := range currentUser.Follows {
// 		if followedID == followedUserId {
// 			isFollowing = true
// 			break
// 		}
// 	}

// 	// Return the updated follow status in the response
// 	response := map[string]bool{"isFollowing": isFollowing} // Toggle status
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusOK)
// 	json.NewEncoder(w).Encode(response)
// }

// func getFollowers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	claims, ok := r.Context().Value(userIDKey).(*Claims)
// 	if !ok {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}
// 	userID := claims.UserID

// 	userKey := fmt.Sprintf("user:%s:followers", userID)
// 	cachedFollowers, err := RdxGet(userKey)
// 	if err == nil {
// 		w.Header().Set("Content-Type", "application/json")
// 		w.Write([]byte(cachedFollowers))
// 		return
// 	}

// 	var user User
// 	err = userCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&user)
// 	if err != nil {
// 		http.Error(w, "User not found", http.StatusNotFound)
// 		return
// 	}

// 	followers := []User{}
// 	for _, followerID := range user.Followers {
// 		var follower User
// 		if err := userCollection.FindOne(context.TODO(), bson.M{"userid": followerID}).Decode(&follower); err == nil {
// 			followers = append(followers, follower)
// 		}
// 	}

// 	// Cache the followers list for the user
// 	followersJSON, _ := json.Marshal(followers)
// 	RdxSet(userKey, string(followersJSON))

// 	json.NewEncoder(w).Encode(followers)
// }

// func getFollowing(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	claims, ok := r.Context().Value(userIDKey).(*Claims)
// 	if !ok {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}
// 	userID := claims.UserID

// 	var user User
// 	err := userCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&user)
// 	if err != nil {
// 		http.Error(w, "User not found", http.StatusNotFound)
// 		return
// 	}

// 	following := []User{}
// 	for _, followingID := range user.Follows {
// 		var followUser User
// 		if err := userCollection.FindOne(context.TODO(), bson.M{"userid": followingID}).Decode(&followUser); err == nil {
// 			following = append(following, followUser)
// 		}
// 	}

// 	json.NewEncoder(w).Encode(following)
// }

// func updateFollowRelationship(currentUserID, targetUserID primitive.ObjectID, action string) error {
// func updateFollowRelationship(currentUserID, targetUserID string, action string) error {
// 	var update bson.M
// 	if action == "follow" {
// 		update = bson.M{
// 			"$addToSet": bson.M{
// 				"followers": currentUserID,
// 			},
// 		}
// 	} else if action == "unfollow" {
// 		update = bson.M{
// 			"$pull": bson.M{
// 				"followers": currentUserID,
// 			},
// 		}
// 	} else {
// 		return fmt.Errorf("invalid action: %s", action)
// 	}
// 	_, err := userCollection.UpdateOne(context.TODO(), bson.M{"_id": targetUserID}, update)
// 	if err != nil {
// 		return fmt.Errorf("failed to update target user: %w", err)
// 	}
// 	return nil
// }

// func updateFollowRelationship(currentUser *User, targetUserID string, action string) error {
// 	var update bson.M

// 	if action == "follow" {
// 		// Add target user to the current user's follows if not already present
// 		if !contains(currentUser.Follows, targetUserID) {
// 			currentUser.Follows = append(currentUser.Follows, targetUserID)
// 		}

// 		// Ensure current user is added to target user's followers only once
// 		update = bson.M{"$addToSet": bson.M{"followers": currentUser.UserID}}
// 	} else if action == "unfollow" {
// 		// Remove target user from the current user's follows
// 		currentUser.Follows = removeString(currentUser.Follows, targetUserID)

// 		// Remove current user from target user's followers
// 		update = bson.M{"$pull": bson.M{"followers": currentUser.UserID}}
// 	} else {
// 		return fmt.Errorf("invalid action")
// 	}

// 	// Update target user's followers
// 	_, err := userCollection.UpdateOne(context.TODO(), bson.M{"userid": targetUserID}, update)
// 	if err != nil {
// 		return fmt.Errorf("error updating followers: %w", err)
// 	}

// 	// Update current user's follows
// 	_, err = userCollection.UpdateOne(context.TODO(), bson.M{"userid": currentUser.UserID}, bson.M{
// 		"$set": bson.M{"follows": currentUser.Follows},
// 	})
// 	if err != nil {
// 		return fmt.Errorf("error updating follows: %w", err)
// 	}

// 	return nil
// }

// func updateFollowRelationship(currentUser *User, targetUserID string, action string) error {
// 	var update bson.M
// 	if action == "follow" {
// 		currentUser.Follows = append(currentUser.Follows, targetUserID)
// 		update = bson.M{"$addToSet": bson.M{"followers": currentUser.UserID}}
// 	} else if action == "unfollow" {
// 		currentUser.Follows = removeString(currentUser.Follows, targetUserID)
// 		update = bson.M{"$pull": bson.M{"followers": currentUser.UserID}}
// 	} else {
// 		return fmt.Errorf("invalid action")
// 	}

// 	// Update target user's followers
// 	_, err := userCollection.UpdateOne(context.TODO(), bson.M{"userid": targetUserID}, update)
// 	if err != nil {
// 		return fmt.Errorf("error updating followers: %w", err)
// 	}

// 	// Update current user's follows
// 	_, err = userCollection.UpdateOne(context.TODO(), bson.M{"userid": currentUser.UserID}, bson.M{
// 		"$set": bson.M{"follows": currentUser.Follows},
// 	})
// 	if err != nil {
// 		return fmt.Errorf("error updating follows: %w", err)
// 	}

// 	return nil
// }

// func handleFollowAction(w http.ResponseWriter, r *http.Request, ps httprouter.Params, action string) {
// 	claims, ok := r.Context().Value(userIDKey).(*Claims)
// 	if !ok {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}
// 	// currentUserID, err := primitive.ObjectIDFromHex(claims.UserID)
// 	// if err != nil {
// 	// 	http.Error(w, "Invalid user ID", http.StatusBadRequest)
// 	// 	return
// 	// }
// 	currentUserID := claims.UserID
// 	// targetUserID, err := primitive.ObjectIDFromHex(ps.ByName("id"))
// 	// if err != nil {
// 	// 	http.Error(w, "Invalid target user ID", http.StatusBadRequest)
// 	// 	return
// 	// }
// 	targetUserID := ps.ByName("id")

// 	// action := r.URL.Query().Get("action")
// 	// if action != "follow" && action != "unfollow" {
// 	// 	http.Error(w, "Invalid action", http.StatusBadRequest)
// 	// 	return
// 	// }

// 	if err := updateFollowRelationship(currentUserID, targetUserID, action); err != nil {
// 		log.Printf("Error updating follow relationship: %v", err)
// 		http.Error(w, "Failed to update follow relationship", http.StatusInternalServerError)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(map[string]bool{"isFollowing": action == "follow", "ok": true})
// }

// func handleFollowAction(w http.ResponseWriter, r *http.Request, ps httprouter.Params, action string) {
// 	claims, ok := r.Context().Value(userIDKey).(*Claims)
// 	if !ok {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}
// 	userID := claims.UserID
// 	targetUserID := ps.ByName("id")
// 	if targetUserID == "" {
// 		http.Error(w, "User ID is required", http.StatusBadRequest)
// 		return
// 	}

// 	// Retrieve the current user
// 	var currentUser User
// 	err := userCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&currentUser)
// 	if err != nil {
// 		http.Error(w, "User not found", http.StatusNotFound)
// 		return
// 	}

// 	// Perform follow/unfollow action
// 	err = updateFollowRelationship(&currentUser, targetUserID, action)
// 	if err != nil {
// 		log.Printf("Error updating relationship: %v", err)
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// Response
// 	isFollowing := action == "follow"
// 	response := map[string]bool{"isFollowing": isFollowing}
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusOK)
// 	json.NewEncoder(w).Encode(response)
// }

// func toggleFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	handleFollowAction(w, r, ps, "follow")
// }

// func toggleUnFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	handleFollowAction(w, r, ps, "unfollow")
// }

// func toggleFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	handleFollowAction(w, r, ps)
// }

// func toggleUnFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	handleFollowAction(w, r, ps)
// }

// func paginateResults(ctx context.Context, collection *mongo.Collection, filter bson.M, page, limit int) ([]User, error) {
// 	skip := (page - 1) * limit
// 	options := options.Find().SetSkip(int64(skip)).SetLimit(int64(limit))
// 	cursor, err := collection.Find(ctx, filter, options)
// 	if err != nil {
// 		return nil, err
// 	}
// 	var results []User
// 	if err := cursor.All(ctx, &results); err != nil {
// 		return nil, err
// 	}
// 	return results, nil
// }
