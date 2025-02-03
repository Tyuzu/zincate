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

func DoesFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

func GetFollowers(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

func GetFollowing(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

func UpdateFollowRelationship(currentUserID, targetUserID, action string) error {
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

func HandleFollowAction(w http.ResponseWriter, r *http.Request, ps httprouter.Params, action string) {
	claims, ok := r.Context().Value(userIDKey).(*Claims)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	currentUserID := claims.UserID
	targetUserID := ps.ByName("id")

	if err := UpdateFollowRelationship(currentUserID, targetUserID, action); err != nil {
		log.Printf("Error updating follow relationship: %v", err)
		http.Error(w, "Failed to update follow relationship", http.StatusInternalServerError)
		return
	}

	SetUserData(action, targetUserID, currentUserID)

	response := map[string]any{
		"isFollowing": action == "follow",
		"ok":          true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func ToggleFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	HandleFollowAction(w, r, ps, "follow")
}

func ToggleUnFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	HandleFollowAction(w, r, ps, "unfollow")
}
