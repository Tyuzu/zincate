package feed

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/mq"
	"naevis/profile"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
)

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
		mediaPaths, mediaNames, err = saveUploadedFiles(r, "images", "image", newPost.PostID, newPost.UserID)
		if err != nil {
			http.Error(w, "Failed to upload images: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if len(mediaPaths) == 0 && len(mediaNames) == 0 {
			http.Error(w, "No media uploaded", http.StatusBadRequest)
			return
		}

	case "video":
		mediaRes, mediaPaths, mediaNames, err = saveUploadedVideoFile(r, "videos", newPost.PostID, newPost.UserID)
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
	m := mq.Index{EntityType: "feedpost", EntityId: newPost.PostID, Method: "POST"}
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
