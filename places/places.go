package places

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/mq"
	"naevis/rdx"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func CreatePlace(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Parse the multipart form with a 10 MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Retrieve and validate place data
	name := r.FormValue("name")
	address := r.FormValue("address")
	description := r.FormValue("description")
	capacity := r.FormValue("capacity")
	category := r.FormValue("category")

	if name == "" || address == "" || description == "" || capacity == "" || category == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	// Validate capacity
	cap, err := strconv.Atoi(capacity)
	if err != nil || cap <= 0 {
		http.Error(w, "Capacity must be a positive integer", http.StatusBadRequest)
		return
	}

	// Retrieve the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		log.Println("Failed to retrieve user ID from context")
		return
	}

	// Create the Place object
	place := structs.Place{
		Name:        name,
		Address:     address,
		Description: description,
		Category:    category,
		Capacity:    cap,
		PlaceID:     utils.GenerateID(14),
		CreatedBy:   requestingUserID,
		CreatedAt:   time.Now(),
		ReviewCount: 0,
		// CreatedAt:   time.Now().Format(time.RFC3339),
	}

	// Handle banner file upload
	bannerFile, header, err := r.FormFile("banner")
	if err != nil && err != http.ErrMissingFile {
		http.Error(w, "Error retrieving banner file", http.StatusBadRequest)
		return
	}

	if bannerFile != nil {
		defer bannerFile.Close()

		// // Validate MIME type (e.g., image/jpeg, image/png)
		// mimeType := header.Header.Get("Content-Type")
		// if mimeType != "image/jpeg" && mimeType != "image/png" && mimeType != "image/webp" {
		// 	http.Error(w, "Invalid banner file type. Only JPEG, PNG and Webp are allowed.", http.StatusBadRequest)
		// 	return
		// }

		if !utils.ValidateImageFileType(w, header) {
			http.Error(w, "Invalid banner file type. Only jpeg, png, webp, gif, bmp, tiff are allowed.", http.StatusBadRequest)
			return
		}

		// Ensure the directory exists
		bannerDir := "./static/placepic"
		if err := os.MkdirAll(bannerDir, os.ModePerm); err != nil {
			http.Error(w, "Error creating directory for banner", http.StatusInternalServerError)
			return
		}

		// Save the banner image
		// bannerPath := fmt.Sprintf("%s/%s.jpg", bannerDir, place.PlaceID)
		bannerPath := fmt.Sprintf("%s/%s.webp", bannerDir, place.PlaceID)
		out, err := os.Create(bannerPath)
		if err != nil {
			http.Error(w, "Error saving banner", http.StatusInternalServerError)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, bannerFile); err != nil {
			os.Remove(bannerPath) // Clean up partial files
			http.Error(w, "Error saving banner", http.StatusInternalServerError)
			return
		}

		// place.Banner = fmt.Sprintf("%s.jpg", place.PlaceID)
		place.Banner = fmt.Sprintf("%s.webp", place.PlaceID)
	}

	// Insert the place into MongoDB
	_, err = db.PlacesCollection.InsertOne(context.TODO(), place)
	if err != nil {
		log.Printf("Error inserting place: %v", err)
		http.Error(w, "Error creating place", http.StatusInternalServerError)
		return
	}

	userdata.SetUserData("place", place.PlaceID, requestingUserID)

	m := mq.Index{EntityType: "place", EntityId: place.PlaceID, Method: "POST"}
	go mq.Emit("place-created", m)

	// Respond with the created place
	w.WriteHeader(http.StatusCreated)
	sanitizedPlace := map[string]any{
		"placeid":     place.PlaceID,
		"name":        place.Name,
		"address":     place.Address,
		"description": place.Description,
		"category":    place.Category,
		"capacity":    place.Capacity,
		"banner":      place.Banner,
		"created_by":  place.CreatedBy,
	}

	if err := json.NewEncoder(w).Encode(sanitizedPlace); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetPlaces(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	// // Check if places are cached
	// cachedPlaces, err := rdx.RdxGet("places")
	// if err == nil && cachedPlaces != "" {
	// 	// Return cached places if available
	// 	w.Write([]byte(cachedPlaces))
	// 	return
	// }

	cursor, err := db.PlacesCollection.Find(context.TODO(), bson.M{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var places []structs.Place
	if err = cursor.All(context.TODO(), &places); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Cache the result
	placesJSON, _ := json.Marshal(places)
	rdx.RdxSet("places", string(placesJSON))

	if places == nil {
		places = []structs.Place{}
	}

	// Encode and return places data
	json.NewEncoder(w).Encode(places)
}

func GetPlace(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("placeid")

	// Aggregation pipeline to fetch place along with related tickets, media, and merch
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "placeid", Value: id}}}},
	}

	// Execute the aggregation query
	cursor, err := db.PlacesCollection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var place structs.Place
	if cursor.Next(context.TODO()) {
		if err := cursor.Decode(&place); err != nil {
			http.Error(w, "Failed to decode place data", http.StatusInternalServerError)
			return
		}
	} else {
		// http.Error(w, "Place not found", http.StatusNotFound)
		// Respond with success
		w.WriteHeader(http.StatusNotFound)
		response := map[string]any{
			"status":  http.StatusNoContent,
			"message": "Place not found",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Encode the place as JSON and write to response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(place); err != nil {
		http.Error(w, "Failed to encode place data", http.StatusInternalServerError)
	}
}

func EditPlace(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeID := ps.ByName("placeid")

	// Retrieve the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusUnauthorized)
		return
	}
	log.Println("Requesting User ID:", requestingUserID)

	// Get the existing place from the database
	var place structs.Place
	err := db.PlacesCollection.FindOne(context.TODO(), bson.M{"placeid": placeID}).Decode(&place)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Place not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Ensure the requesting user is the creator of the place
	if place.CreatedBy != requestingUserID {
		http.Error(w, "You are not authorized to edit this place", http.StatusForbidden)
		return
	}

	// Parse the multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Prepare fields for update
	updateFields := bson.M{}
	if name := r.FormValue("name"); name != "" {
		updateFields["name"] = name
	}
	if address := r.FormValue("address"); address != "" {
		updateFields["address"] = address
	}
	if description := r.FormValue("description"); description != "" {
		updateFields["description"] = description
	}

	// Validate that at least one field is being updated
	if len(updateFields) == 0 {
		http.Error(w, "No valid fields to update", http.StatusBadRequest)
		return
	}

	// Handle banner file upload
	bannerFile, header, err := r.FormFile("banner")
	if err != nil && err != http.ErrMissingFile {
		http.Error(w, "Error retrieving banner file", http.StatusBadRequest)
		return
	}
	if bannerFile != nil {
		defer bannerFile.Close()

		// // Validate MIME type
		// mimeType := header.Header.Get("Content-Type")
		// if mimeType != "image/jpeg" && mimeType != "image/png" {
		// 	http.Error(w, "Invalid banner file type. Only JPEG and PNG are allowed.", http.StatusBadRequest)
		// 	return
		// }

		if !utils.ValidateImageFileType(w, header) {
			http.Error(w, "Invalid banner file type. Only jpeg, png, webp, gif, bmp, tiff are allowed.", http.StatusBadRequest)
			return
		}

		// Ensure the directory exists
		bannerDir := "./static/placepic"
		if err := os.MkdirAll(bannerDir, os.ModePerm); err != nil {
			http.Error(w, "Error creating directory for banner", http.StatusInternalServerError)
			return
		}

		// Save the banner file
		// bannerPath := fmt.Sprintf("%s/%s.jpg", bannerDir, placeID)
		bannerPath := fmt.Sprintf("%s/%s.webp", bannerDir, placeID)
		out, err := os.Create(bannerPath)
		if err != nil {
			http.Error(w, "Error saving banner", http.StatusInternalServerError)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, bannerFile); err != nil {
			os.Remove(bannerPath) // Clean up partial files
			http.Error(w, "Error saving banner", http.StatusInternalServerError)
			return
		}

		// Add banner to update fields
		// updateFields["banner"] = fmt.Sprintf("%s.jpg", placeID)
		updateFields["banner"] = fmt.Sprintf("%s.webp", placeID)
	}

	// Update the `updated_at` field
	updateFields["updated_at"] = time.Now().Format(time.RFC3339)

	// Update the place in the database
	_, err = db.PlacesCollection.UpdateOne(context.TODO(), bson.M{"placeid": placeID}, bson.M{"$set": updateFields})
	if err != nil {
		http.Error(w, "Error updating place", http.StatusInternalServerError)
		return
	}

	// Invalidate cache (log success or failure)
	if _, err := rdx.RdxDel("place:" + placeID); err != nil {
		log.Printf("Cache deletion failed for place ID: %s. Error: %v", placeID, err)
	} else {
		log.Printf("Cache successfully invalidated for place ID: %s", placeID)
	}

	// // Respond with updated fields
	// w.Header().Set("Content-Type", "application/json")
	// w.WriteHeader(http.StatusOK)
	// if err := json.NewEncoder(w).Encode(updateFields); err != nil {
	// 	http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	// }

	m := mq.Index{EntityType: "place", EntityId: placeID, Method: "PUT"}
	go mq.Emit("place-edited", m)

	// Respond with the created place
	w.WriteHeader(http.StatusCreated)
	sanitizedPlace := map[string]any{
		"placeid":     place.PlaceID,
		"name":        place.Name,
		"address":     place.Address,
		"description": place.Description,
		"category":    place.Category,
		"capacity":    place.Capacity,
		"banner":      place.Banner,
		"created_by":  place.CreatedBy,
	}
	if err := json.NewEncoder(w).Encode(sanitizedPlace); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func DeletePlace(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeID := ps.ByName("placeid")
	var place structs.Place

	// Get the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}
	// log.Println("Requesting User ID:", requestingUserID)

	// Get the place from the database using placeID
	err := db.PlacesCollection.FindOne(context.TODO(), bson.M{"placeid": placeID}).Decode(&place)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Place not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Check if the place was created by the requesting user
	if place.CreatedBy != requestingUserID {
		http.Error(w, "You are not authorized to delete this place", http.StatusForbidden)
		return
	}

	// Delete the place from MongoDB
	_, err = db.PlacesCollection.DeleteOne(context.TODO(), bson.M{"placeid": placeID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rdx.RdxDel("place:" + placeID) // Invalidate the cache for the deleted place

	userdata.DelUserData("place", placeID, requestingUserID)

	m := mq.Index{EntityType: "place", EntityId: placeID, Method: "DELETE"}
	go mq.Emit("place-deleted", m)

	// Respond with success
	w.WriteHeader(http.StatusOK)
	response := map[string]any{
		"status":  http.StatusNoContent,
		"message": "Place deleted successfully",
	}
	json.NewEncoder(w).Encode(response)
}
