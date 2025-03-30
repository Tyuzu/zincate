package feed

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/structs"
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

// func CreateTweetPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	tokenString := r.Header.Get("Authorization")
// 	claims, err := profile.ValidateJWT(tokenString)
// 	if err != nil {
// 		log.Printf("JWT validation error: %v", err) // Log the error for debugging
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	// Parse multipart form data (20 MB limit)
// 	if err := r.ParseMultipartForm(20 << 20); err != nil {
// 		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
// 		return
// 	}

// 	userid := claims.UserID
// 	username := claims.Username

// 	// Extract post content and type
// 	postType := r.FormValue("type")
// 	postText := r.FormValue("text")

// 	// Validate post type
// 	validPostTypes := map[string]bool{"text": true, "image": true, "video": true, "blog": true, "merchandise": true}
// 	if !validPostTypes[postType] {
// 		http.Error(w, "Invalid post type", http.StatusBadRequest)
// 		return
// 	}

// 	newPost := structs.Post{
// 		PostID:    utils.GenerateID(12),
// 		Username:  username,
// 		UserID:    userid,
// 		Text:      postText,
// 		Timestamp: time.Now().Format(time.RFC3339),
// 		Likes:     0,
// 		Type:      postType,
// 	}

// 	var mediaPaths []string
// 	var mediaNames []string
// 	var mediaRes []int

// 	// if postType == "text" && len(postText) == 0 {
// 	// 	http.Error(w, "Text post must have content", http.StatusBadRequest)
// 	// 	return
// 	// }

// 	// Handle different post types
// 	switch postType {
// 	case "image":
// 		mediaPaths, mediaNames, err = saveUploadedFiles(r, "images", "image")
// 		if err != nil {
// 			http.Error(w, "Failed to upload images: "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}
// 		if len(mediaPaths) == 0 && len(mediaNames) == 0 {
// 			http.Error(w, "No media uploaded", http.StatusBadRequest)
// 			return
// 		}

// 	case "video":
// 		mediaRes, mediaPaths, mediaNames, err = saveUploadedVideoFile(r, "videos")
// 		if err != nil {
// 			http.Error(w, "Failed to upload videos: "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}
// 		if len(mediaPaths) == 0 && len(mediaNames) == 0 {
// 			http.Error(w, "No media uploaded", http.StatusBadRequest)
// 			return
// 		}
// 	}

// 	newPost.Resolutions = mediaRes // Store only available resolutions
// 	newPost.MediaURL = mediaNames
// 	newPost.Media = mediaPaths

// 	// Save post in the database
// 	_, err = db.PostsCollection.InsertOne(context.TODO(), newPost)
// 	if err != nil {
// 		http.Error(w, "Failed to insert post into DB: "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	userdata.SetUserData("feedpost", newPost.PostID, userid)
// 	m := mq.Index{EntityType: "feedpost", EntityId: newPost.PostID, Method: "POST"}
// 	go mq.Emit("post-created", m)

// 	// Respond with success
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusOK)
// 	json.NewEncoder(w).Encode(map[string]any{
// 		"ok":      true,
// 		"message": "Post created successfully",
// 		"data":    newPost,
// 	})
// }

// // Helper function to convert a string to ObjectID
// func objectIDFromString(id string) (any, error) {
// 	return primitive.ObjectIDFromHex(id)
// }
