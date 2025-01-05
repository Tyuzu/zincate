package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Function to handle fetching the feed
func getPosts(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	collection := client.Database("twitterClone").Collection("posts")

	// Create an empty slice to store posts
	var posts []Post

	// Filter to fetch all posts (can be adjusted if you need specific filtering)
	filter := bson.M{} // Empty filter for fetching all posts

	// Create the sort order (descending by timestamp)
	sortOrder := bson.D{{Key: "timestamp", Value: -1}}

	// Use the context with timeout to handle long queries and ensure sorting by timestamp
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch posts with sorting options
	cursor, err := collection.Find(ctx, filter, &options.FindOptions{
		Sort: sortOrder, // Apply sorting by timestamp descending
	})
	if err != nil {
		http.Error(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	// Loop through the cursor and decode each post into the `posts` slice
	for cursor.Next(ctx) {
		var post Post
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
		posts = []Post{}
	}

	// Return the list of posts as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"data": posts,
	})
}

// Directory to store uploaded images/videos
const uploadDir = "./postpic/"

func createTweetPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := validateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse multipart form data (20 MB limit)
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	userid := claims.UserID

	username, _ := RdxHget("users", userid)

	// Extract post content and type
	postType := r.FormValue("type")
	postText := r.FormValue("text")

	// Validate post type
	validPostTypes := map[string]bool{"text": true, "image": true, "video": true, "blog": true, "merchandise": true}
	if !validPostTypes[postType] {
		http.Error(w, "Invalid post type", http.StatusBadRequest)
		return
	}

	newPost := Post{
		Username:  username,
		UserID:    userid,
		Text:      postText,
		Timestamp: time.Now().Format(time.RFC3339),
		Likes:     0,
		Type:      postType,
	}

	var mediaPaths []string
	// var err error
	// Handle different post types
	switch postType {
	case "image":
		mediaPaths, err = saveUploadedFiles(r, "images", "image")
		if err != nil {
			http.Error(w, "Failed to upload images: "+err.Error(), http.StatusInternalServerError)
			return
		}
	case "video":
		mediaPaths, err = saveUploadedFiles(r, "videos", "video")
		if err != nil {
			http.Error(w, "Failed to upload videos: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	newPost.Media = mediaPaths

	// Save post in the database
	postsCollection := client.Database("twitterClone").Collection("posts")
	insertResult, err := postsCollection.InsertOne(context.TODO(), newPost)
	if err != nil {
		http.Error(w, "Failed to insert post into DB: "+err.Error(), http.StatusInternalServerError)
		return
	}

	newPost.ID = insertResult.InsertedID

	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Post created successfully",
		"data":    newPost,
	})
}

// saveUploadedFiles handles saving uploaded files and returns their paths
func saveUploadedFiles(r *http.Request, formKey, fileType string) ([]string, error) {
	files := r.MultipartForm.File[formKey]
	if len(files) == 0 {
		return nil, nil // No files to process
	}

	var savedPaths []string
	for _, file := range files {
		// Open uploaded file
		src, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open %s file: %w", fileType, err)
		}
		defer src.Close()

		// Generate a unique file name
		uniqueID := generateID(16)
		// ext := filepath.Ext(file.Filename)
		sanitizedExt := ".jpg" // Default extension
		if fileType == "video" {
			sanitizedExt = ".mp4"
		}
		fileName := uniqueID + sanitizedExt
		dstFilePath := filepath.Join(uploadDir, fileName)

		// Ensure upload directory exists
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create upload directory: %w", err)
		}

		// Save the file
		dst, err := os.Create(dstFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to save %s file: %w", fileType, err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return nil, fmt.Errorf("failed to write %s file: %w", fileType, err)
		}

		// Add relative path to media paths
		savedPaths = append(savedPaths, "/postpic/"+fileName)
	}

	return savedPaths, nil
}

func editPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	postID := ps.ByName("id")
	if postID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	// Parse and validate the incoming JSON
	var updatedPost Post
	if err := json.NewDecoder(r.Body).Decode(&updatedPost); err != nil {
		http.Error(w, "Invalid JSON input: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate fields to be updated
	if updatedPost.Text == "" && len(updatedPost.Media) == 0 && updatedPost.Type == "" {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	// Convert postID to an ObjectID
	id, err := primitive.ObjectIDFromHex(postID)
	if err != nil {
		http.Error(w, "Invalid Post ID format", http.StatusBadRequest)
		return
	}

	claims, ok := r.Context().Value(userIDKey).(*Claims)
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
	postsCollection := client.Database("twitterClone").Collection("posts")
	var existingPost Post
	err = postsCollection.FindOne(context.TODO(), bson.M{"_id": id}).Decode(&existingPost)
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
	result, err := postsCollection.UpdateOne(context.TODO(), bson.M{"_id": id}, update)
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

	// Respond with the updated post
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Post updated successfully",
		"data":    existingPost,
	})
}

// deletePost handles deleting a post by ID
func deletePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	postID := ps.ByName("id")

	if postID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	// Convert postID to ObjectID
	id, err := objectIDFromString(postID)
	if err != nil {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	// Delete the post from MongoDB
	postsCollection := client.Database("twitterClone").Collection("posts")
	result, err := postsCollection.DeleteOne(context.TODO(), bson.M{"_id": id})
	if err != nil {
		http.Error(w, "Failed to delete post", http.StatusInternalServerError)
		return
	}

	if result.DeletedCount == 0 {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Post deleted successfully",
	})
}

// Helper function to convert a string to ObjectID
func objectIDFromString(id string) (interface{}, error) {
	return primitive.ObjectIDFromHex(id)
}
