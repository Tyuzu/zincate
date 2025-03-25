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

var bannerDir string = "./static/placepic"

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

// Handles file upload and returns the banner file name
func handleBannerUpload(w http.ResponseWriter, r *http.Request, placeID string) (string, error) {
	bannerFile, header, err := r.FormFile("banner")
	if err != nil {
		if err == http.ErrMissingFile {
			return "", nil // No file uploaded, continue without it
		}
		return "", fmt.Errorf("error retrieving banner file")
	}
	defer bannerFile.Close()

	if !utils.ValidateImageFileType(w, header) {
		return "", fmt.Errorf("invalid banner file type. Only jpeg, png, webp, gif, bmp, tiff are allowed")
	}

	// Ensure the directory exists
	// bannerDir := "./static/placepic"
	if err := os.MkdirAll(bannerDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("error creating directory for banner")
	}

	// Save the banner image
	bannerPath := fmt.Sprintf("%s/%s.jpg", bannerDir, placeID)
	out, err := os.Create(bannerPath)
	if err != nil {
		return "", fmt.Errorf("error saving banner")
	}
	defer out.Close()

	if _, err := io.Copy(out, bannerFile); err != nil {
		os.Remove(bannerPath) // Cleanup partial files
		return "", fmt.Errorf("error saving banner")
	}

	return fmt.Sprintf("%s.jpg", placeID), nil
}

// Parses and validates form data for places
func parsePlaceFormData(_ http.ResponseWriter, r *http.Request) (structs.Place, error) {
	err := r.ParseMultipartForm(10 << 20) // 10MB limit
	if err != nil {
		return structs.Place{}, fmt.Errorf("unable to parse form")
	}

	name, address, description, category, capacityStr := r.FormValue("name"), r.FormValue("address"), r.FormValue("description"), r.FormValue("category"), r.FormValue("capacity")

	if name == "" || address == "" || description == "" || category == "" || capacityStr == "" {
		return structs.Place{}, fmt.Errorf("all fields are required")
	}

	capacity, err := strconv.Atoi(capacityStr)
	if err != nil || capacity <= 0 {
		return structs.Place{}, fmt.Errorf("capacity must be a positive integer")
	}

	return structs.Place{
		Name:        name,
		Address:     address,
		Description: description,
		Category:    category,
		Capacity:    capacity,
		PlaceID:     utils.GenerateID(14),
		CreatedAt:   time.Now(),
		ReviewCount: 0,
	}, nil
}

// Sends a JSON response
func respondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// Inserts or updates a place in the database
func updatePlaceInDB(w http.ResponseWriter, placeID string, updateFields bson.M) error {
	_, err := db.PlacesCollection.UpdateOne(context.TODO(), bson.M{"placeid": placeID}, bson.M{"$set": updateFields})
	if err != nil {
		http.Error(w, "Error updating place", http.StatusInternalServerError)
		return err
	}

	// Invalidate cache
	if _, err := rdx.RdxDel("place:" + placeID); err != nil {
		log.Printf("Cache deletion failed for place ID: %s. Error: %v", placeID, err)
	} else {
		log.Printf("Cache successfully invalidated for place ID: %s", placeID)
	}

	return nil
}

// Creates a new place
func CreatePlace(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	place, err := parsePlaceFormData(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Retrieve user ID
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}
	place.CreatedBy = requestingUserID

	// Handle banner upload
	banner, err := handleBannerUpload(w, r, place.PlaceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	place.Banner = banner

	// Insert into MongoDB
	_, err = db.PlacesCollection.InsertOne(context.TODO(), place)
	if err != nil {
		http.Error(w, "Error creating place", http.StatusInternalServerError)
		return
	}

	utils.CreateThumb(place.PlaceID, bannerDir, ".jpg", 300, 200)

	userdata.SetUserData("place", place.PlaceID, requestingUserID)
	go mq.Emit("place-created", mq.Index{EntityType: "place", EntityId: place.PlaceID, Method: "POST"})

	respondWithJSON(w, http.StatusCreated, place)
}

// Edits an existing place
func EditPlace(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeID := ps.ByName("placeid")

	// Retrieve user ID
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusUnauthorized)
		return
	}

	// Fetch the existing place
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

	// Ensure authorization
	if place.CreatedBy != requestingUserID {
		http.Error(w, "You are not authorized to edit this place", http.StatusForbidden)
		return
	}

	// Parse form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Collect update fields
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

	// Handle banner upload
	banner, err := handleBannerUpload(w, r, placeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if banner != "" {
		updateFields["banner"] = banner
	}

	// Update database
	updateFields["updated_at"] = time.Now()
	if err := updatePlaceInDB(w, placeID, updateFields); err != nil {
		return
	}

	utils.CreateThumb(placeID, bannerDir, ".jpg", 300, 200)

	go mq.Emit("place-edited", mq.Index{EntityType: "place", EntityId: placeID, Method: "PUT"})

	respondWithJSON(w, http.StatusOK, updateFields)
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
