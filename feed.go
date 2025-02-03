package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"naevis/mq"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	var posts []Post

	// Filter to fetch all posts (can be adjusted if you need specific filtering)
	filter := bson.M{} // Empty filter for fetching all posts

	// Create the sort order (descending by timestamp)
	sortOrder := bson.D{{Key: "timestamp", Value: -1}}

	// Use the context with timeout to handle long queries and ensure sorting by timestamp
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch posts with sorting options
	cursor, err := postsCollection.Find(ctx, filter, &options.FindOptions{
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

func GetPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("postid")

	// Aggregation pipeline to fetch post along with related tickets, media, and merch
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "postid", Value: id}}}},
	}

	// Execute the aggregation query
	cursor, err := postsCollection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var post Post
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

// Directory to store uploaded images/videos
const feedVideoUploadDir = "./postpic/"

func CreateTweetPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

	// username, _ := RdxHget("users", userid)
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

	newPost := Post{
		PostID:    generateID(12),
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
		mediaPaths, err = saveUploadedVideoFile(r, "videos")
		if err != nil {
			http.Error(w, "Failed to upload videos: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	newPost.Media = mediaPaths

	// Save post in the database
	// postsCollection := client.Database("eventdb").Collection("posts")
	// insertResult, err := postsCollection.InsertOne(context.TODO(), newPost)
	_, err = postsCollection.InsertOne(context.TODO(), newPost)
	if err != nil {
		http.Error(w, "Failed to insert post into DB: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// newPost.ID = insertResult.InsertedID

	SetUserData("feedpost", newPost.PostID, userid)

	mq.Emit("post-created")

	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Post created successfully",
		"data":    newPost,
	})
}
func saveUploadedVideoFile(r *http.Request, formKey string) ([]string, error) {
	files := r.MultipartForm.File[formKey]
	if len(files) == 0 {
		return nil, nil // No file to process
	}

	// Process the first file only
	file := files[0]
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open video file: %w", err)
	}
	defer src.Close()

	// Generate unique filename
	uniqueID := generateID(16)
	originalFileName := uniqueID + ".mp4"
	originalFilePath := filepath.Join(feedVideoUploadDir, originalFileName)

	// Ensure upload directory exists
	if err := os.MkdirAll(feedVideoUploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Save the original file
	if err := saveFile(src, originalFilePath); err != nil {
		return nil, fmt.Errorf("failed to save video file: %w", err)
	}

	// Generate video resolutions
	resolutions := map[string]string{
		"144p": "256x144",
		"480p": "854x480",
		"720p": "1280x720",
	}

	var highestResolutionPath string
	for label, size := range resolutions {
		outputFileName := fmt.Sprintf("%s-%s.mp4", uniqueID, label)
		outputFilePath := filepath.Join(feedVideoUploadDir, outputFileName)
		outputPosterName := fmt.Sprintf("%s-%s.jpg", uniqueID, label)
		outputPosterPath := filepath.Join(feedVideoUploadDir, outputPosterName)

		if err := processVideoResolution(originalFilePath, outputFilePath, size); err != nil {
			return nil, fmt.Errorf("failed to create %s video: %w", label, err)
		}
		fmt.Printf("Video file %s created successfully!\n", outputFileName)

		if err := createVideoPoster(outputFilePath, outputPosterPath); err != nil {
			return nil, fmt.Errorf("failed to create %s poster: %w", label, err)
		}
		fmt.Printf("Poster file %s created successfully!\n", outputPosterName)

		highestResolutionPath = "/postpic/" + outputFileName
	}

	// Generate a default poster from the original video (at 5 seconds)
	defaultPosterPath := filepath.Join(feedVideoUploadDir, fmt.Sprintf("%s.jpg", uniqueID))
	if err := createPoster(originalFilePath, defaultPosterPath); err != nil {
		return nil, fmt.Errorf("failed to create default video poster: %w", err)
	}
	fmt.Printf("Default poster %s created successfully!\n", defaultPosterPath)

	// Generate subtitles asynchronously
	go createSubtitleFile(uniqueID)

	// Notify MQ system about the uploaded video
	mq.Emit("postpics-uploaded")

	return []string{highestResolutionPath}, nil
}

// Saves the uploaded file to disk
func saveFile(src io.Reader, dstPath string) error {
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// Processes video into a specific resolution using FFMPEG
func processVideoResolution(inputPath, outputPath, size string) error {
	cmd := exec.Command(
		"ffmpeg", "-i", inputPath,
		"-vf", fmt.Sprintf("scale=%s", size),
		"-c:v", "libx264", "-crf", "23",
		"-preset", "veryfast",
		"-c:a", "aac", "-b:a", "128k",
		"-movflags", "+faststart",
		outputPath,
	)

	return cmd.Run()
}

// Creates a poster (poster) from a video at 5 seconds
func createPoster(videoPath, posterPath string) error {
	cmd := exec.Command(
		"ffmpeg", "-i", videoPath,
		"-ss", "00:00:01", "-vframes", "1",
		"-q:v", "2", posterPath,
	)
	return cmd.Run()
}

// Creates a poster for specific resolutions
func createVideoPoster(inputPath, outputPath string) error {
	cmd := exec.Command(
		"ffmpeg", "-i", inputPath,
		"-ss", "00:00:01", "-vframes", "1",
		"-q:v", "2", outputPath,
	)

	return cmd.Run()
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
		dstFilePath := filepath.Join(feedVideoUploadDir, fileName)

		// Ensure upload directory exists
		if err := os.MkdirAll(feedVideoUploadDir, 0755); err != nil {
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

	mq.Emit("postpics-uploaded")

	return savedPaths, nil
}

func createSubtitleFile(uniqueID string) {
	// Example subtitles
	subtitles := []Subtitle{
		{
			Index:   1,
			Start:   "00:00:00.000",
			End:     "00:00:01.000",
			Content: "Welcome to the video!",
		},
		{
			Index:   2,
			Start:   "00:00:01.001",
			End:     "00:00:02.000",
			Content: "In this video, we'll learn how to create subtitles in Go.",
		},
		{
			Index:   3,
			Start:   "00:00:02.001",
			End:     "00:00:03.000",
			Content: "Let's get started!",
		},
	}

	var lang = "english"

	// File name for the .vtt file
	// fileName := "example.vtt"
	fileName := fmt.Sprintf("./postpic/%s-%s.vtt", uniqueID, lang)

	// Create the VTT file
	err := createVTTFile(fileName, subtitles)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Subtitle file %s created successfully!\n", fileName)
}

// Subtitle represents a single subtitle entry
type Subtitle struct {
	Index   int
	Start   string // Start time in format "hh:mm:ss.mmm"
	End     string // End time in format "hh:mm:ss.mmm"
	Content string // Subtitle text
}

func createVTTFile(fileName string, subtitles []Subtitle) error {
	// Create or overwrite the file
	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	// Write the WebVTT header
	_, err = file.WriteString("WEBVTT\n\n")
	if err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	// Write each subtitle entry
	for _, subtitle := range subtitles {
		entry := fmt.Sprintf("%d\n%s --> %s\n%s\n\n",
			subtitle.Index,
			subtitle.Start,
			subtitle.End,
			subtitle.Content,
		)
		_, err := file.WriteString(entry)
		if err != nil {
			return fmt.Errorf("failed to write subtitle entry: %v", err)
		}
	}

	return nil
}

func EditPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	postID := ps.ByName("postid")
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

	// // Convert postID to an ObjectID
	// id, err := primitive.ObjectIDFromHex(postID)
	// if err != nil {
	// 	http.Error(w, "Invalid Post ID format", http.StatusBadRequest)
	// 	return
	// }

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
	// postsCollection := client.Database("eventdb").Collection("posts")
	var existingPost Post
	err := postsCollection.FindOne(context.TODO(), bson.M{"postid": postID}).Decode(&existingPost)
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
	result, err := postsCollection.UpdateOne(context.TODO(), bson.M{"postid": postID}, update)
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

	mq.Emit("post-edited")

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
func DeletePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	postID := ps.ByName("postid")

	if postID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	// Retrieve the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(userIDKey).(string)
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
	// postsCollection := client.Database("eventdb").Collection("posts")
	result, err := postsCollection.DeleteOne(context.TODO(), bson.M{"postid": postID})
	if err != nil {
		http.Error(w, "Failed to delete post", http.StatusInternalServerError)
		return
	}

	if result.DeletedCount == 0 {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	DelUserData("feedpost", postID, requestingUserID)

	mq.Emit("post-deleted")

	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Post deleted successfully",
	})
}

// // Helper function to convert a string to ObjectID
// func objectIDFromString(id string) (interface{}, error) {
// 	return primitive.ObjectIDFromHex(id)
// }
