package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func addMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	if entityID == "" {
		http.Error(w, "Entity ID is required", http.StatusBadRequest)
		return
	}

	err := r.ParseMultipartForm(50 << 20) // Limit to 50 MB
	if err != nil {
		http.Error(w, "Unable to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	requestingUserID, ok := r.Context().Value(userIDKey).(string)
	if !ok || requestingUserID == "" {
		http.Error(w, "Invalid or missing user ID", http.StatusUnauthorized)
		return
	}

	media := Media{
		EntityID:   entityID,
		EntityType: entityType,
		Caption:    r.FormValue("caption"),
		CreatorID:  requestingUserID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if entityType == "event" {
		media.ID = "e" + generateID(16)
	} else {
		media.ID = "p" + generateID(16)
	}

	file, fileHeader, err := r.FormFile("media")
	if err != nil {
		if err == http.ErrMissingFile {
			http.Error(w, "Media file is required", http.StatusBadRequest)
		} else {
			http.Error(w, "Error retrieving media file: "+err.Error(), http.StatusBadRequest)
		}
		return
	}
	if file != nil {
		defer file.Close()
	}

	var fileExtension, mimeType string
	if file != nil {
		mimeType = fileHeader.Header.Get("Content-Type")
		switch {
		case strings.HasPrefix(mimeType, "image/"):
			fileExtension = ".jpg"
			media.Type = "image"
		case strings.HasPrefix(mimeType, "video/"):
			fileExtension = ".mp4"
			media.Type = "video"
		default:
			http.Error(w, "Unsupported media type", http.StatusUnsupportedMediaType)
			return
		}

		savePath := "./uploads/" + media.ID + fileExtension
		out, err := os.Create(savePath)
		if err != nil {
			http.Error(w, "Error saving media file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, file); err != nil {
			http.Error(w, "Error saving media file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		media.URL = media.ID + fileExtension
		media.MimeType = mimeType
		if media.Type == "video" {
			media.FileSize = fileHeader.Size
			media.Duration = extractVideoDuration(savePath)
		}
	}

	collection := userCollection
	_, err = collection.InsertOne(r.Context(), media)
	if err != nil {
		http.Error(w, "Error saving media to database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}

func getMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	mediaID := ps.ByName("id")

	collection := userCollection
	var media Media
	err := collection.FindOne(r.Context(), bson.M{
		"entityid":   entityID,
		"entitytype": entityType,
		"id":         mediaID,
	}).Decode(&media)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Media not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}

func getMedias(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")

	collection := userCollection
	cursor, err := collection.Find(r.Context(), bson.M{
		"entityid":   entityID,
		"entitytype": entityType,
	})
	if err != nil {
		http.Error(w, "Failed to retrieve media", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(r.Context())

	var medias []Media
	if err = cursor.All(r.Context(), &medias); err != nil {
		http.Error(w, "Failed to parse media", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(medias)
}

func deleteMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	mediaID := ps.ByName("id")

	collection := userCollection
	_, err := collection.DeleteOne(r.Context(), bson.M{
		"entityid":   entityID,
		"entitytype": entityType,
		"id":         mediaID,
	})
	if err != nil {
		http.Error(w, "Failed to delete media", http.StatusInternalServerError)
		return
	}

	// Respond with success
	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"status":  http.StatusNoContent,
		"message": "Media deleted successfully",
	}
	json.NewEncoder(w).Encode(response)
}

func extractVideoDuration(savePath string) int {
	_ = savePath
	return 5
}

func editMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	mediaID := ps.ByName("id")
	cacheKey := fmt.Sprintf("media:%s:%s", entityID, mediaID)

	// Check the cache first
	cachedMedia, err := RdxGet(cacheKey)
	if err == nil && cachedMedia != "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cachedMedia))
		return
	}

	// Fetch the media from MongoDB
	var media Media
	collection := userCollection
	err = collection.FindOne(r.Context(), bson.M{
		"entityid":   entityID,
		"entitytype": entityType,
		"id":         mediaID,
	}).Decode(&media)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Media not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Cache the result
	mediaJSON, _ := json.Marshal(media)
	RdxSet(cacheKey, string(mediaJSON))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}
