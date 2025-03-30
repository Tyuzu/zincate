package feed

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/globals"
	"naevis/middleware"
	"naevis/mq"
	"naevis/structs"
	"naevis/userdata"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func EditPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	postID := ps.ByName("postid")
	if postID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	// Parse and validate the incoming JSON
	var updatedPost structs.Post
	if err := json.NewDecoder(r.Body).Decode(&updatedPost); err != nil {
		http.Error(w, "Invalid JSON input: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate fields to be updated
	if updatedPost.Text == "" && len(updatedPost.Media) == 0 && updatedPost.Type == "" {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	// // Convert postID to an ObjectID
	// id, err := primitive.ObjectIDFromHex(postID)
	// if err != nil {
	// 	http.Error(w, "Invalid Post ID format", http.StatusBadRequest)
	// 	return
	// }

	claims, ok := r.Context().Value(globals.UserIDKey).(*middleware.Claims)
	if !ok || claims.UserID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := claims.UserID
	if userID == "" {
		http.Error(w, "Unauthorized: Missing user ID", http.StatusUnauthorized)
		return
	}

	// Check ownership of the post
	// db.PostsCollection := client.Database("eventdb").Collection("posts")
	var existingPost structs.Post
	err := db.PostsCollection.FindOne(context.TODO(), bson.M{"postid": postID}).Decode(&existingPost)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Post not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching post: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if existingPost.UserID != userID {
		http.Error(w, "Unauthorized: You can only edit your own posts", http.StatusForbidden)
		return
	}

	// Prepare the update document
	updateFields := bson.M{}
	if updatedPost.Text != "" {
		updateFields["text"] = updatedPost.Text
	}
	if len(updatedPost.Media) > 0 {
		updateFields["media"] = updatedPost.Media
	}
	if updatedPost.Type != "" {
		updateFields["type"] = updatedPost.Type
	}
	updateFields["timestamp"] = time.Now().Format(time.RFC3339) // Always update timestamp on edit

	update := bson.M{"$set": updateFields}

	// Perform the update operation
	result, err := db.PostsCollection.UpdateOne(context.TODO(), bson.M{"postid": postID}, update)
	if err != nil {
		http.Error(w, "Failed to update post: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if result.MatchedCount == 0 {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// Update the in-memory representation for response
	if updatedPost.Text != "" {
		existingPost.Text = updatedPost.Text
	}
	if len(updatedPost.Media) > 0 {
		existingPost.Media = updatedPost.Media
	}
	if updatedPost.Type != "" {
		existingPost.Type = updatedPost.Type
	}
	existingPost.Timestamp = updateFields["timestamp"].(string)

	m := mq.Index{EntityType: "feedpost", EntityId: postID, Method: "PUT"}
	go mq.Emit("post-edited", m)

	// Respond with the updated post
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Post updated successfully",
		"data":    existingPost,
	})
}

// deletePost handles deleting a post by ID
func DeletePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	postID := ps.ByName("postid")

	if postID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	// Retrieve the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	// // Convert postID to ObjectID
	// id, err := objectIDFromString(postID)
	// if err != nil {
	// 	http.Error(w, "Invalid post ID", http.StatusBadRequest)
	// 	return
	// }

	// Delete the post from MongoDB
	// db.PostsCollection := client.Database("eventdb").Collection("posts")
	result, err := db.PostsCollection.DeleteOne(context.TODO(), bson.M{"postid": postID})
	if err != nil {
		http.Error(w, "Failed to delete post", http.StatusInternalServerError)
		return
	}

	// Check if the file exists
	var existingFile FileMetadata
	db.FilesCollection.FindOne(context.TODO(), bson.M{"postid": postID}).Decode(&existingFile)

	RemoveUserFile(requestingUserID, postID, existingFile.Hash)

	if result.DeletedCount == 0 {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	userdata.DelUserData("feedpost", postID, requestingUserID)

	m := mq.Index{EntityType: "feedpost", EntityId: postID, Method: "DELETE"}
	go mq.Emit("post-deleted", m)

	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Post deleted successfully",
	})
}
