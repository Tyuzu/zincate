package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// Function to handle the creation of merchandise
func createMerch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	if eventID == "" {
		http.Error(w, "Event ID is required", http.StatusBadRequest)
		return
	}

	// Parse the multipart form data (with a 10MB limit)
	err := r.ParseMultipartForm(10 << 20) // Limit the size to 10 MB
	if err != nil {
		http.Error(w, "Unable to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Retrieve form values
	name := r.FormValue("name")
	price, err := strconv.ParseFloat(r.FormValue("price"), 64)
	if err != nil || price <= 0 {
		http.Error(w, "Invalid price value. Must be a positive number.", http.StatusBadRequest)
		return
	}

	stock, err := strconv.Atoi(r.FormValue("stock"))
	if err != nil || stock < 0 {
		http.Error(w, "Invalid stock value. Must be a non-negative integer.", http.StatusBadRequest)
		return
	}

	// Validate merchandise data
	if len(name) == 0 || len(name) > 100 {
		http.Error(w, "Name must be between 1 and 100 characters.", http.StatusBadRequest)
		return
	}

	// Create a new Merch instance
	merch := Merch{
		EventID:   eventID,
		Name:      name,
		Price:     price,
		Stock:     stock,
		MerchID:   generateID(14), // Generate unique merchandise ID
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Handle banner file upload
	bannerFile, bannerHeader, err := r.FormFile("image")
	if err != nil && err != http.ErrMissingFile {
		http.Error(w, "Error retrieving banner file: "+err.Error(), http.StatusBadRequest)
		return
	}

	if bannerFile != nil {
		defer bannerFile.Close()

		// Validate file type using MIME type
		mimeType := bannerHeader.Header.Get("Content-Type")
		fileExtension := ""
		switch mimeType {
		case "image/jpeg":
			fileExtension = ".jpg"
		case "image/png":
			fileExtension = ".png"
		default:
			http.Error(w, "Unsupported image type. Only JPEG and PNG are allowed.", http.StatusUnsupportedMediaType)
			return
		}

		// Save the banner file securely
		savePath := "./merchpic/" + merch.MerchID + fileExtension
		out, err := os.Create(savePath)
		if err != nil {
			http.Error(w, "Error saving banner: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, bannerFile); err != nil {
			http.Error(w, "Error saving banner: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Set the banner photo URL
		merch.MerchPhoto = merch.MerchID + fileExtension
	}

	// Insert merchandise into MongoDB
	collection := client.Database("eventdb").Collection("merch")
	_, err = collection.InsertOne(context.TODO(), merch)
	if err != nil {
		http.Error(w, "Failed to insert merchandise: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with the created merchandise
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"message": "Merchandise created successfully.",
		"data":    merch,
	})
}

// Fetch a single merchandise item
func getMerch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	merchID := ps.ByName("merchid")
	cacheKey := fmt.Sprintf("merch:%s:%s", eventID, merchID)

	// Check if the merch is cached
	cachedMerch, err := RdxGet(cacheKey)
	if err == nil && cachedMerch != "" {
		// Return cached data
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cachedMerch))
		return
	}

	collection := client.Database("eventdb").Collection("merch")
	var merch Merch
	err = collection.FindOne(context.TODO(), bson.M{"eventid": eventID, "merchid": merchID}).Decode(&merch)
	if err != nil {
		http.Error(w, fmt.Sprintf("Merchandise not found: %v", err), http.StatusNotFound)
		return
	}

	// Cache the result
	merchJSON, _ := json.Marshal(merch)
	RdxSet(cacheKey, string(merchJSON))

	// Respond with merch data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(merch)
}

// Fetch a list of merchandise items
func getMerchs(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	cacheKey := fmt.Sprintf("merchlist:%s", eventID)

	// Check if the merch list is cached
	cachedMerchs, err := RdxGet(cacheKey)
	if err == nil && cachedMerchs != "" {
		// Return cached list
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cachedMerchs))
		return
	}

	collection := client.Database("eventdb").Collection("merch")
	var merchList []Merch
	filter := bson.M{"eventid": eventID}

	cursor, err := collection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Failed to fetch merchandise", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		var merch Merch
		if err := cursor.Decode(&merch); err != nil {
			http.Error(w, "Failed to decode merchandise", http.StatusInternalServerError)
			return
		}
		merchList = append(merchList, merch)
	}

	if err := cursor.Err(); err != nil {
		http.Error(w, "Cursor error", http.StatusInternalServerError)
		return
	}

	if len(merchList) == 0 {
		merchList = []Merch{}
	}

	// Cache the list
	merchListJSON, _ := json.Marshal(merchList)
	RdxSet(cacheKey, string(merchListJSON))

	// Respond with the list of merch
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(merchList)
}

// Edit a merchandise item
func editMerch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	merchID := ps.ByName("merchid")

	// Parse the request body
	var merch Merch
	if err := json.NewDecoder(r.Body).Decode(&merch); err != nil {
		http.Error(w, "Invalid input data", http.StatusBadRequest)
		return
	}

	// Validate merch data
	if merch.Name == "" || merch.Price <= 0 || merch.Stock < 0 {
		http.Error(w, "Invalid merchandise data: Name, Price, and Stock are required.", http.StatusBadRequest)
		return
	}

	// Prepare update data
	updateFields := bson.M{}
	if merch.Name != "" {
		updateFields["name"] = merch.Name
	}
	if merch.Price > 0 {
		updateFields["price"] = merch.Price
	}
	if merch.Stock >= 0 {
		updateFields["stock"] = merch.Stock
	}

	// Update the merch in MongoDB
	collection := client.Database("eventdb").Collection("merch")
	updateResult, err := collection.UpdateOne(
		context.TODO(),
		bson.M{"eventid": eventID, "merchid": merchID},
		bson.M{"$set": updateFields},
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update merchandise: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if update was successful
	if updateResult.MatchedCount == 0 {
		http.Error(w, "Merchandise not found", http.StatusNotFound)
		return
	}

	// Invalidate the specific merch cache
	RdxDel(fmt.Sprintf("merch:%s:%s", eventID, merchID))

	// Send response
	// w.Header().Set("Content-Type", "application/json")
	// w.WriteHeader(http.StatusOK)
	// json.NewEncoder(w).Encode("Merchandise updated successfully")
	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Merch updated successfully",
	})
}

// Delete a merchandise item
func deleteMerch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	merchID := ps.ByName("merchid")

	// Delete the merch from MongoDB
	collection := client.Database("eventdb").Collection("merch")
	deleteResult, err := collection.DeleteOne(context.TODO(), bson.M{"eventid": eventID, "merchid": merchID})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete merchandise: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if delete was successful
	if deleteResult.DeletedCount == 0 {
		http.Error(w, "Merchandise not found", http.StatusNotFound)
		return
	}

	// Invalidate the cache
	RdxDel(fmt.Sprintf("merch:%s:%s", eventID, merchID))

	// // Send response
	// w.WriteHeader(http.StatusOK)
	// w.Write([]byte("Merchandise deleted successfully"))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Merch updated successfully",
	})
}

func buyMerch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	merchID := ps.ByName("merchid")

	// Parse the request body to extract quantity
	var requestData struct {
		Quantity int `json:"quantity"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil || requestData.Quantity <= 0 {
		http.Error(w, "Invalid quantity", http.StatusBadRequest)
		return
	}

	// Find the merch in the database
	collection := client.Database("eventdb").Collection("merch")
	var merch Merch // Define the Merch struct based on your schema
	err = collection.FindOne(context.TODO(), bson.M{"eventid": eventID, "merchid": merchID}).Decode(&merch)
	if err != nil {
		http.Error(w, "Merch not found or other error", http.StatusNotFound)
		return
	}

	// Check if there are enough merch available for purchase
	if merch.Stock < requestData.Quantity {
		http.Error(w, "Not enough merch available for purchase", http.StatusBadRequest)
		return
	}

	// Decrease the merch stock by the requested quantity
	update := bson.M{"$inc": bson.M{"stock": -requestData.Quantity}}
	_, err = collection.UpdateOne(context.TODO(), bson.M{"eventid": eventID, "merchid": merchID}, update)
	if err != nil {
		http.Error(w, "Failed to update merch stock", http.StatusInternalServerError)
		return
	}

	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Successfully purchased %d of %s", requestData.Quantity, merch.Name),
	})
}
