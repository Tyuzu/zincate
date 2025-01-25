package main

import (
	"context"
	"encoding/json"
	"naevis/mq"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Function to handle fetching the feed
func GetBlogPosts(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	// Create an empty slice to store blogposts
	var blogposts []BlogPost

	// Filter to fetch all blogposts (can be adjusted if you need specific filtering)
	filter := bson.M{} // Empty filter for fetching all blogposts

	// Create the sort order (descending by timestamp)
	sortOrder := bson.D{{Key: "timestamp", Value: -1}}

	// Use the context with timeout to handle long queries and ensure sorting by timestamp
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch blogposts with sorting options
	cursor, err := blogCollection.Find(ctx, filter, &options.FindOptions{
		Sort: sortOrder, // Apply sorting by timestamp descending
	})
	if err != nil {
		http.Error(w, "Failed to fetch blogposts", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	// Loop through the cursor and decode each blogpost into the `blogposts` slice
	for cursor.Next(ctx) {
		var blogpost BlogPost
		if err := cursor.Decode(&blogpost); err != nil {
			http.Error(w, "Failed to decode blogpost", http.StatusInternalServerError)
			return
		}
		blogposts = append(blogposts, blogpost)
	}

	// Handle cursor error
	if err := cursor.Err(); err != nil {
		http.Error(w, "Cursor error", http.StatusInternalServerError)
		return
	}

	// If no blogposts found, return an empty array
	if len(blogposts) == 0 {
		blogposts = []BlogPost{}
	}

	// Return the list of blogposts as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":   true,
		"data": blogposts,
	})
}

func GetBlogPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("postid")

	// Aggregation pipeline to fetch blogpost along with related tickets, media, and merch
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "postid", Value: id}}}},
	}

	// Execute the aggregation query
	cursor, err := blogCollection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var blogpost BlogPost
	if cursor.Next(context.TODO()) {
		if err := cursor.Decode(&blogpost); err != nil {
			http.Error(w, "Failed to decode blogpost data", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "BlogPost not found", http.StatusNotFound)
		return
	}

	// Encode the blogpost as JSON and write to response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(blogpost); err != nil {
		http.Error(w, "Failed to encode blogpost data", http.StatusInternalServerError)
	}
}

// deleteBlogPost handles deleting a blogpost by ID
func DeleteBlogPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	blogpostID := ps.ByName("postid")

	if blogpostID == "" {
		http.Error(w, "BlogPost ID is required", http.StatusBadRequest)
		return
	}

	// Retrieve the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(userIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	// // Convert blogpostID to ObjectID
	// id, err := objectIDFromString(blogpostID)
	// if err != nil {
	// 	http.Error(w, "Invalid blogpost ID", http.StatusBadRequest)
	// 	return
	// }

	// Delete the blogpost from MongoDB
	result, err := blogCollection.DeleteOne(context.TODO(), bson.M{"postid": blogpostID})
	if err != nil {
		http.Error(w, "Failed to delete blogpost", http.StatusInternalServerError)
		return
	}

	if result.DeletedCount == 0 {
		http.Error(w, "BlogPost not found", http.StatusNotFound)
		return
	}

	DelUserData("blog", blogpostID, requestingUserID)

	mq.Emit("blogpost-deleted")

	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "BlogPost deleted successfully",
	})
}

func CreateBlogPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := validateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userid := claims.UserID
	username := claims.Username

	// Parse JSON body
	var blogPostData struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		Category string `json:"category"`
		Text     string `json:"text"`
	}

	// Decode the request body into the blogPostData struct
	if err := json.NewDecoder(r.Body).Decode(&blogPostData); err != nil {
		http.Error(w, "Failed to parse JSON body", http.StatusBadRequest)
		return
	}

	// Validate inputs
	if blogPostData.Title == "" || blogPostData.Content == "" {
		http.Error(w, "Title and content are required", http.StatusBadRequest)
		return
	}

	newBlogPost := BlogPost{
		PostID:    generateID(12),
		Username:  username,
		UserID:    userid,
		Text:      blogPostData.Text,
		Title:     blogPostData.Title,
		Content:   blogPostData.Content,
		Category:  blogPostData.Category,
		Timestamp: time.Now().Format(time.RFC3339),
		Likes:     0,
	}

	// Save blogpost in the database
	_, err = blogCollection.InsertOne(context.TODO(), newBlogPost)
	if err != nil {
		http.Error(w, "Failed to insert blogpost into DB: "+err.Error(), http.StatusInternalServerError)
		return
	}

	SetUserData("blogpost", newBlogPost.PostID, userid)
	mq.Emit("blogpost-created")

	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "BlogPost created successfully",
		"data":    newBlogPost,
	})
}

// func CreateBlogPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	tokenString := r.Header.Get("Authorization")
// 	claims, err := validateJWT(tokenString)
// 	if err != nil {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	// // Parse multipart form data (20 MB limit)
// 	// if err := r.ParseMultipartForm(20 << 20); err != nil {
// 	// 	http.Error(w, "Failed to parse form data", http.StatusBadRequest)
// 	// 	return
// 	// }

// 	userid := claims.UserID
// 	username := claims.Username
// 	fmt.Println("content : ", r.FormValue("content"))
// 	// Extract blogpost content, title, category, and type
// 	// blogpostType := r.FormValue("type")
// 	blogpostText := r.FormValue("text")
// 	blogpostTitle := r.FormValue("title")       // Title from payload
// 	blogpostContent := r.FormValue("content")   // Content from payload
// 	blogpostCategory := r.FormValue("category") // Category from payload

// 	// // Validate blogpost type
// 	// validBlogPostTypes := map[string]bool{"text": true, "image": true, "video": true, "blog": true, "merchandise": true}
// 	// if !validBlogPostTypes[blogpostType] {
// 	// 	http.Error(w, "Invalid blogpost type", http.StatusBadRequest)
// 	// 	return
// 	// }

// 	newBlogPost := BlogPost{
// 		PostID:    generateID(12),
// 		Username:  username,
// 		UserID:    userid,
// 		Text:      blogpostText,
// 		Title:     blogpostTitle,    // Store the title
// 		Content:   blogpostContent,  // Store the content
// 		Category:  blogpostCategory, // Store the category
// 		Timestamp: time.Now().Format(time.RFC3339),
// 		Likes:     0,
// 		// Type:      blogpostType,
// 	}

// 	// var mediaPaths []string
// 	// // Handle different blogpost types
// 	// switch blogpostType {
// 	// case "image":
// 	// 	mediaPaths, err = saveUploadedFiles(r, "images", "image")
// 	// 	if err != nil {
// 	// 		http.Error(w, "Failed to upload images: "+err.Error(), http.StatusInternalServerError)
// 	// 		return
// 	// 	}
// 	// case "video":
// 	// 	mediaPaths, err = saveUploadedFiles(r, "videos", "video")
// 	// 	if err != nil {
// 	// 		http.Error(w, "Failed to upload videos: "+err.Error(), http.StatusInternalServerError)
// 	// 		return
// 	// 	}
// 	// }

// 	// newBlogPost.Media = mediaPaths

// 	// Save blogpost in the database
// 	_, err = blogCollection.InsertOne(context.TODO(), newBlogPost)
// 	if err != nil {
// 		http.Error(w, "Failed to insert blogpost into DB: "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	SetUserData("blogpost", newBlogPost.PostID, userid)
// 	mq.Emit("blogpost-created")

// 	// Respond with success
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusOK)
// 	json.NewEncoder(w).Encode(map[string]interface{}{
// 		"ok":      true,
// 		"message": "BlogPost created successfully",
// 		"data":    newBlogPost,
// 	})
// }

func EditBlogPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	blogpostID := ps.ByName("postid")
	if blogpostID == "" {
		http.Error(w, "BlogPost ID is required", http.StatusBadRequest)
		return
	}

	// Parse and validate the incoming JSON
	var updatedBlogPost BlogPost
	if err := json.NewDecoder(r.Body).Decode(&updatedBlogPost); err != nil {
		http.Error(w, "Invalid JSON input: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate fields to be updated
	if updatedBlogPost.Text == "" && len(updatedBlogPost.Media) == 0 && updatedBlogPost.Type == "" && updatedBlogPost.Title == "" && updatedBlogPost.Content == "" && updatedBlogPost.Category == "" {
		http.Error(w, "No fields to update", http.StatusBadRequest)
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

	// Check ownership of the blogpost
	var existingBlogPost BlogPost
	err := blogCollection.FindOne(context.TODO(), bson.M{"postid": blogpostID}).Decode(&existingBlogPost)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "BlogPost not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching blogpost: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if existingBlogPost.UserID != userID {
		http.Error(w, "Unauthorized: You can only edit your own blogposts", http.StatusForbidden)
		return
	}

	// Prepare the update document
	updateFields := bson.M{}
	if updatedBlogPost.Text != "" {
		updateFields["text"] = updatedBlogPost.Text
	}
	if updatedBlogPost.Title != "" {
		updateFields["title"] = updatedBlogPost.Title // Update title
	}
	if updatedBlogPost.Content != "" {
		updateFields["content"] = updatedBlogPost.Content // Update content
	}
	if updatedBlogPost.Category != "" {
		updateFields["category"] = updatedBlogPost.Category // Update category
	}
	if len(updatedBlogPost.Media) > 0 {
		updateFields["media"] = updatedBlogPost.Media
	}
	if updatedBlogPost.Type != "" {
		updateFields["type"] = updatedBlogPost.Type
	}
	updateFields["timestamp"] = time.Now().Format(time.RFC3339) // Always update timestamp on edit

	update := bson.M{"$set": updateFields}

	// Perform the update operation
	result, err := blogCollection.UpdateOne(context.TODO(), bson.M{"postid": blogpostID}, update)
	if err != nil {
		http.Error(w, "Failed to update blogpost: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if result.MatchedCount == 0 {
		http.Error(w, "BlogPost not found", http.StatusNotFound)
		return
	}

	// Update the in-memory representation for response
	if updatedBlogPost.Text != "" {
		existingBlogPost.Text = updatedBlogPost.Text
	}
	if updatedBlogPost.Title != "" {
		existingBlogPost.Title = updatedBlogPost.Title
	}
	if updatedBlogPost.Content != "" {
		existingBlogPost.Content = updatedBlogPost.Content
	}
	if updatedBlogPost.Category != "" {
		existingBlogPost.Category = updatedBlogPost.Category
	}
	if len(updatedBlogPost.Media) > 0 {
		existingBlogPost.Media = updatedBlogPost.Media
	}
	if updatedBlogPost.Type != "" {
		existingBlogPost.Type = updatedBlogPost.Type
	}
	existingBlogPost.Timestamp = updateFields["timestamp"].(string)

	mq.Emit("blogpost-edited")

	// Respond with the updated blogpost
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "BlogPost updated successfully",
		"data":    existingBlogPost,
	})
}

// func CreateBlogPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	tokenString := r.Header.Get("Authorization")
// 	claims, err := validateJWT(tokenString)
// 	if err != nil {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	// Parse multipart form data (20 MB limit)
// 	if err := r.ParseMultipartForm(20 << 20); err != nil {
// 		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
// 		return
// 	}

// 	userid := claims.UserID

// 	// username, _ := RdxHget("users", userid)
// 	username := claims.Username

// 	// Extract blogpost content and type
// 	blogpostType := r.FormValue("type")
// 	blogpostText := r.FormValue("text")

// 	// Validate blogpost type
// 	validBlogPostTypes := map[string]bool{"text": true, "image": true, "video": true, "blog": true, "merchandise": true}
// 	if !validBlogPostTypes[blogpostType] {
// 		http.Error(w, "Invalid blogpost type", http.StatusBadRequest)
// 		return
// 	}

// 	newBlogPost := BlogPost{
// 		PostID:    generateID(12),
// 		Username:  username,
// 		UserID:    userid,
// 		Text:      blogpostText,
// 		Timestamp: time.Now().Format(time.RFC3339),
// 		Likes:     0,
// 		Type:      blogpostType,
// 	}

// 	var mediaPaths []string
// 	// var err error
// 	// Handle different blogpost types
// 	switch blogpostType {
// 	case "image":
// 		mediaPaths, err = saveUploadedFiles(r, "images", "image")
// 		if err != nil {
// 			http.Error(w, "Failed to upload images: "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}
// 	case "video":
// 		mediaPaths, err = saveUploadedFiles(r, "videos", "video")
// 		if err != nil {
// 			http.Error(w, "Failed to upload videos: "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}
// 	}

// 	newBlogPost.Media = mediaPaths

// 	// Save blogpost in the database
// 	// blogCollection := client.Database("eventdb").Collection("blogposts")
// 	// insertResult, err := blogCollection.InsertOne(context.TODO(), newBlogPost)
// 	_, err = blogCollection.InsertOne(context.TODO(), newBlogPost)
// 	if err != nil {
// 		http.Error(w, "Failed to insert blogpost into DB: "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// newBlogPost.ID = insertResult.InsertedID

// 	SetUserData("blogpost", newBlogPost.PostID, userid)

// 	mq.Emit("blogpost-created")

// 	// Respond with success
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusOK)
// 	json.NewEncoder(w).Encode(map[string]interface{}{
// 		"ok":      true,
// 		"message": "BlogPost created successfully",
// 		"data":    newBlogPost,
// 	})
// }

// func EditBlogPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	blogpostID := ps.ByName("postid")
// 	if blogpostID == "" {
// 		http.Error(w, "BlogPost ID is required", http.StatusBadRequest)
// 		return
// 	}

// 	// Parse and validate the incoming JSON
// 	var updatedBlogPost BlogPost
// 	if err := json.NewDecoder(r.Body).Decode(&updatedBlogPost); err != nil {
// 		http.Error(w, "Invalid JSON input: "+err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	// Validate fields to be updated
// 	if updatedBlogPost.Text == "" && len(updatedBlogPost.Media) == 0 && updatedBlogPost.Type == "" {
// 		http.Error(w, "No fields to update", http.StatusBadRequest)
// 		return
// 	}

// 	// // Convert blogpostID to an ObjectID
// 	// id, err := primitive.ObjectIDFromHex(blogpostID)
// 	// if err != nil {
// 	// 	http.Error(w, "Invalid BlogPost ID format", http.StatusBadRequest)
// 	// 	return
// 	// }

// 	claims, ok := r.Context().Value(userIDKey).(*Claims)
// 	if !ok || claims.UserID == "" {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	userID := claims.UserID
// 	if userID == "" {
// 		http.Error(w, "Unauthorized: Missing user ID", http.StatusUnauthorized)
// 		return
// 	}

// 	// Check ownership of the blogpost
// 	// blogCollection := client.Database("eventdb").Collection("blogposts")
// 	var existingBlogPost BlogPost
// 	err := blogCollection.FindOne(context.TODO(), bson.M{"blogpostid": blogpostID}).Decode(&existingBlogPost)
// 	if err != nil {
// 		if err == mongo.ErrNoDocuments {
// 			http.Error(w, "BlogPost not found", http.StatusNotFound)
// 		} else {
// 			http.Error(w, "Error fetching blogpost: "+err.Error(), http.StatusInternalServerError)
// 		}
// 		return
// 	}
// 	if existingBlogPost.UserID != userID {
// 		http.Error(w, "Unauthorized: You can only edit your own blogposts", http.StatusForbidden)
// 		return
// 	}

// 	// Prepare the update document
// 	updateFields := bson.M{}
// 	if updatedBlogPost.Text != "" {
// 		updateFields["text"] = updatedBlogPost.Text
// 	}
// 	if len(updatedBlogPost.Media) > 0 {
// 		updateFields["media"] = updatedBlogPost.Media
// 	}
// 	if updatedBlogPost.Type != "" {
// 		updateFields["type"] = updatedBlogPost.Type
// 	}
// 	updateFields["timestamp"] = time.Now().Format(time.RFC3339) // Always update timestamp on edit

// 	update := bson.M{"$set": updateFields}

// 	// Perform the update operation
// 	result, err := blogCollection.UpdateOne(context.TODO(), bson.M{"postid": blogpostID}, update)
// 	if err != nil {
// 		http.Error(w, "Failed to update blogpost: "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	if result.MatchedCount == 0 {
// 		http.Error(w, "BlogPost not found", http.StatusNotFound)
// 		return
// 	}

// 	// Update the in-memory representation for response
// 	if updatedBlogPost.Text != "" {
// 		existingBlogPost.Text = updatedBlogPost.Text
// 	}
// 	if len(updatedBlogPost.Media) > 0 {
// 		existingBlogPost.Media = updatedBlogPost.Media
// 	}
// 	if updatedBlogPost.Type != "" {
// 		existingBlogPost.Type = updatedBlogPost.Type
// 	}
// 	existingBlogPost.Timestamp = updateFields["timestamp"].(string)

// 	mq.Emit("blogpost-edited")

// 	// Respond with the updated blogpost
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusOK)
// 	json.NewEncoder(w).Encode(map[string]interface{}{
// 		"ok":      true,
// 		"message": "BlogPost updated successfully",
// 		"data":    existingBlogPost,
// 	})
// }

// // Helper function to convert a string to ObjectID
// func objectIDFromString(id string) (interface{}, error) {
// 	return primitive.ObjectIDFromHex(id)
// }
