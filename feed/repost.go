package feed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"naevis/db"
	"naevis/profile"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RepostRequest defines the request format
type RepostRequest struct {
	PostID  string `json:"postId"`
	Content string `json:"content,omitempty"` // Optional comment
}

// RepostHandler handles reposting an existing post
func Repost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	claims, err := profile.ValidateJWT(r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID, username := claims.UserID, claims.Username

	var req RepostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.PostID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	// Check if the original post exists
	originalPostID, err := primitive.ObjectIDFromHex(req.PostID)
	if err != nil {
		http.Error(w, "Invalid Post ID", http.StatusBadRequest)
		return
	}

	collection := db.PostsCollection
	var originalPost bson.M
	err = collection.FindOne(r.Context(), bson.M{"_id": originalPostID}).Decode(&originalPost)
	if err != nil {
		http.Error(w, "Original post not found", http.StatusNotFound)
		return
	}

	// Prevent duplicate reposts by the same user
	existingRepost := collection.FindOne(r.Context(), bson.M{"userId": userID, "repostOf": req.PostID})
	if existingRepost.Err() == nil {
		http.Error(w, "You already reposted this", http.StatusConflict)
		return
	}

	// Create a new repost entry
	newRepost := bson.M{
		"userId":      userID,
		"username":    username,
		"content":     req.Content,
		"repostOf":    req.PostID,
		"repostCount": 0,
		"createdAt":   time.Now(),
	}

	insertResult, err := collection.InsertOne(r.Context(), newRepost)
	if err != nil {
		http.Error(w, "Failed to repost", http.StatusInternalServerError)
		return
	}

	// Increment the repost count in the original post
	_, err = collection.UpdateOne(r.Context(), bson.M{"_id": originalPostID}, bson.M{"$inc": bson.M{"repostCount": 1}})
	if err != nil {
		fmt.Println("Failed to update repost count:", err)
	}

	// Return response
	resp := map[string]interface{}{
		"message": "Post reposted successfully",
		"postId":  insertResult.InsertedID,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func GetFeed(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	collection := db.PostsCollection
	cursor, err := collection.Find(r.Context(), bson.M{})
	if err != nil {
		http.Error(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(r.Context())

	var posts []bson.M
	for cursor.Next(r.Context()) {
		var post bson.M
		if err := cursor.Decode(&post); err == nil {
			// If it's a repost, fetch original post details
			if post["repostOf"] != nil {
				originalPostID := post["repostOf"].(string)
				var originalPost bson.M
				err := collection.FindOne(r.Context(), bson.M{"postid": originalPostID}).Decode(&originalPost)
				if err == nil {
					post["originalPost"] = originalPost
				}
			}
			posts = append(posts, post)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

func DeleteRepost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	claims, err := profile.ValidateJWT(r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := claims.UserID

	postID := r.URL.Query().Get("postId")
	if postID == "" {
		http.Error(w, "Post ID required", http.StatusBadRequest)
		return
	}

	collection := db.PostsCollection

	// Check if the post is a repost
	var repost bson.M
	err = collection.FindOne(r.Context(), bson.M{"_id": postID, "userId": userID}).Decode(&repost)
	if err != nil {
		http.Error(w, "Repost not found", http.StatusNotFound)
		return
	}

	// Delete the repost
	_, err = collection.DeleteOne(r.Context(), bson.M{"_id": postID})
	if err != nil {
		http.Error(w, "Failed to delete repost", http.StatusInternalServerError)
		return
	}

	// Decrease original post's repost count
	originalPostID := repost["repostOf"].(string)
	collection.UpdateOne(r.Context(), bson.M{"_id": originalPostID}, bson.M{"$inc": bson.M{"repostCount": -1}})

	w.WriteHeader(http.StatusNoContent)
}
