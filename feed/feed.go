package feed

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/middleware"
	"naevis/mq"
	"naevis/profile"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Function to handle fetching the feed
func GetPosts(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// collection := client.Database("eventdb").Collection("posts")

	// Create an empty slice to store posts
	var posts []structs.Post

	// Filter to fetch all posts (can be adjusted if you need specific filtering)
	filter := bson.M{} // Empty filter for fetching all posts

	// Create the sort order (descending by timestamp)
	sortOrder := bson.D{{Key: "timestamp", Value: -1}}

	// Use the context with timeout to handle long queries and ensure sorting by timestamp
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch posts with sorting options
	cursor, err := db.PostsCollection.Find(ctx, filter, &options.FindOptions{
		Sort: sortOrder, // Apply sorting by timestamp descending
	})
	if err != nil {
		http.Error(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	// Loop through the cursor and decode each post into the `posts` slice
	for cursor.Next(ctx) {
		var post structs.Post
		if err := cursor.Decode(&post); err != nil {
			http.Error(w, "Failed to decode post", http.StatusInternalServerError)
			return
		}
		posts = append(posts, post)
	}

	// Handle cursor error
	if err := cursor.Err(); err != nil {
		http.Error(w, "Cursor error", http.StatusInternalServerError)
		return
	}

	// If no posts found, return an empty array
	if len(posts) == 0 {
		posts = []structs.Post{}
	}

	// Return the list of posts as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"ok":   true,
		"data": posts,
	})
}

func GetPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("postid")

	// Aggregation pipeline to fetch post along with related tickets, media, and merch
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "postid", Value: id}}}},
	}

	// Execute the aggregation query
	cursor, err := db.PostsCollection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var post structs.Post
	if cursor.Next(context.TODO()) {
		if err := cursor.Decode(&post); err != nil {
			http.Error(w, "Failed to decode post data", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// Encode the post as JSON and write to response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		http.Error(w, "Failed to encode post data", http.StatusInternalServerError)
	}
}

func CreateTweetPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := profile.ValidateJWT(tokenString)
	if err != nil {
		log.Printf("JWT validation error: %v", err) // Log the error for debugging
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse multipart form data (20 MB limit)
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	userid := claims.UserID
	username := claims.Username

	// Extract post content and type
	postType := r.FormValue("type")
	postText := r.FormValue("text")

	// Validate post type
	validPostTypes := map[string]bool{"text": true, "image": true, "video": true, "blog": true, "merchandise": true}
	if !validPostTypes[postType] {
		http.Error(w, "Invalid post type", http.StatusBadRequest)
		return
	}

	newPost := structs.Post{
		PostID:    utils.GenerateID(12),
		Username:  username,
		UserID:    userid,
		Text:      postText,
		Timestamp: time.Now().Format(time.RFC3339),
		Likes:     0,
		Type:      postType,
	}

	var mediaPaths []string
	var mediaNames []string
	var mediaRes []int

	// if postType == "text" && len(postText) == 0 {
	// 	http.Error(w, "Text post must have content", http.StatusBadRequest)
	// 	return
	// }

	// Handle different post types
	switch postType {
	case "image":
		mediaPaths, mediaNames, err = saveUploadedFiles(r, "images", "image")
		if err != nil {
			http.Error(w, "Failed to upload images: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if len(mediaPaths) == 0 && len(mediaNames) == 0 {
			http.Error(w, "No media uploaded", http.StatusBadRequest)
			return
		}

	case "video":
		mediaRes, mediaPaths, mediaNames, err = saveUploadedVideoFile(r, "videos")
		if err != nil {
			http.Error(w, "Failed to upload videos: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if len(mediaPaths) == 0 && len(mediaNames) == 0 {
			http.Error(w, "No media uploaded", http.StatusBadRequest)
			return
		}
	}

	newPost.Resolutions = mediaRes // Store only available resolutions
	newPost.MediaURL = mediaNames
	newPost.Media = mediaPaths

	// Save post in the database
	_, err = db.PostsCollection.InsertOne(context.TODO(), newPost)
	if err != nil {
		http.Error(w, "Failed to insert post into DB: "+err.Error(), http.StatusInternalServerError)
		return
	}

	userdata.SetUserData("feedpost", newPost.PostID, userid)
	m := mq.Index{EntityType: "feedpost", EntityId: newPost.PostID, Action: "POST"}
	go mq.Emit("post-created", m)

	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Post created successfully",
		"data":    newPost,
	})
}

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

	m := mq.Index{EntityType: "feedpost", EntityId: postID, Action: "PUT"}
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

	if result.DeletedCount == 0 {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	userdata.DelUserData("feedpost", postID, requestingUserID)

	m := mq.Index{EntityType: "feedpost", EntityId: postID, Action: "DELETE"}
	go mq.Emit("post-deleted", m)

	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Post deleted successfully",
	})
}

// // Helper function to convert a string to ObjectID
// func objectIDFromString(id string) (any, error) {
// 	return primitive.ObjectIDFromHex(id)
// }
