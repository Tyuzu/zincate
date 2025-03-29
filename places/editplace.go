package places

import (
	"context"
	"encoding/json"
	"log"
	"naevis/autocom"
	"naevis/db"
	"naevis/globals"
	"naevis/mq"
	"naevis/rdx"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

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
		autocom.AddPlaceToAutocorrect(rdx.Conn, placeID, name)
	}
	if address := r.FormValue("address"); address != "" {
		updateFields["address"] = address
	}
	if description := r.FormValue("description"); description != "" {
		updateFields["description"] = description
	}
	if category := r.FormValue("category"); category != "" {
		updateFields["category"] = category
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
