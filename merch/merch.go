package merch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
)

var merchUploadPath string = "./static/merchpic"

// Function to handle the creation of merchandise
func CreateMerch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	entityType := ps.ByName("entityType")
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
	merch := structs.Merch{
		// EventID:    eventID,
		EntityType: entityType,
		EntityID:   eventID,
		Name:       name,
		Price:      price,
		Stock:      stock,
		MerchID:    utils.GenerateID(14), // Generate unique merchandise ID
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
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
		case "image/webp":
			fileExtension = ".webp"
		case "image/png":
			fileExtension = ".png"
		default:
			http.Error(w, "Unsupported image type. Only JPG, PNG and WEBP are allowed.", http.StatusUnsupportedMediaType)
			return
		}

		// Save the banner file securely
		savePath := merchUploadPath + "/" + merch.MerchID + fileExtension
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
		utils.CreateThumb(merch.MerchID, merchUploadPath, ".jpg", 150, 200)

		// Set the banner photo URL
		merch.MerchPhoto = merch.MerchID + fileExtension
	}

	// Insert merchandise into MongoDB
	// collection := client.Database("eventdb").Collection("merch")
	_, err = db.MerchCollection.InsertOne(context.TODO(), merch)
	if err != nil {
		http.Error(w, "Failed to insert merchandise: "+err.Error(), http.StatusInternalServerError)
		return
	}

	m := mq.Index{EntityType: "merch", EntityId: merch.MerchID, Method: "POST", ItemType: "event", ItemId: eventID}
	go mq.Emit("merch-created", m)

	// Respond with the created merchandise
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Merchandise created successfully.",
		"data":    merch,
	})
}

// Fetch a single merchandise item
func GetMerch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	merchID := ps.ByName("merchid")
	entityType := ps.ByName("entityType")

	fmt.Println(entityType, eventID, merchID)

	// cacheKey := fmt.Sprintf("merch:%s:%s", eventID, merchID)

	// // Check if the merch is cached
	// cachedMerch, err := rdx.RdxGet(cacheKey)
	// if err == nil && cachedMerch != "" {
	// 	// Return cached data
	// 	w.Header().Set("Content-Type", "application/json")
	// 	w.Write([]byte(cachedMerch))
	// 	return
	// }

	// collection := client.Database("eventdb").Collection("merch")
	var merch structs.Merch
	err := db.MerchCollection.FindOne(context.TODO(), bson.M{"entity_type": entityType, "entity_id": eventID, "merchid": merchID}).Decode(&merch)
	if err != nil {
		// http.Error(w, fmt.Sprintf("Merchandise not found: %v", err), http.StatusNotFound)
		utils.RespondWithJSON(w, 404, "")
		return
	}

	// // Cache the result
	// merchJSON, _ := json.Marshal(merch)
	// rdx.RdxSet(cacheKey, string(merchJSON))

	// Respond with merch data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(merch)
}

// Fetch a list of merchandise items
func GetMerchs(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	entityType := ps.ByName("entityType")
	cacheKey := fmt.Sprintf("merchlist:%s", eventID)

	fmt.Println(eventID, entityType)
	// // Check if the merch list is cached
	// cachedMerchs, err := rdx.RdxGet(cacheKey)
	// if err == nil && cachedMerchs != "" {
	// 	// Return cached list
	// 	w.Header().Set("Content-Type", "application/json")
	// 	w.Write([]byte(cachedMerchs))
	// 	return
	// }

	// collection := client.Database("eventdb").Collection("merch")
	var merchList []structs.Merch
	filter := bson.M{"entity_type": entityType, "entity_id": eventID}

	cursor, err := db.MerchCollection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Failed to fetch merchandise", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		var merch structs.Merch
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
		merchList = []structs.Merch{}
	}

	// Cache the list
	merchListJSON, _ := json.Marshal(merchList)
	rdx.RdxSet(cacheKey, string(merchListJSON))

	// Respond with the list of merch
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(merchList)
}

// Edit a merchandise item
func EditMerch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	merchID := ps.ByName("merchid")
	entityType := ps.ByName("entityType")

	// Parse the request body
	var merch structs.Merch
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
	// collection := client.Database("eventdb").Collection("merch")
	updateResult, err := db.MerchCollection.UpdateOne(
		context.TODO(),
		bson.M{"entity_type": entityType, "entity_id": eventID, "merchid": merchID},
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
	rdx.RdxDel(fmt.Sprintf("merch:%s:%s", eventID, merchID))

	m := mq.Index{EntityType: "merch", EntityId: merchID, Method: "PUT", ItemType: entityType, ItemId: eventID}
	go mq.Emit("merch-edited", m)

	// Send response
	// w.Header().Set("Content-Type", "application/json")
	// w.WriteHeader(http.StatusOK)
	// json.NewEncoder(w).Encode("Merchandise updated successfully")
	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Merch updated successfully",
	})
}

// Delete a merchandise item
func DeleteMerch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	merchID := ps.ByName("merchid")
	entityType := ps.ByName("entityType")

	// Delete the merch from MongoDB
	// collection := client.Database("eventdb").Collection("merch")
	deleteResult, err := db.MerchCollection.DeleteOne(context.TODO(), bson.M{"entity_type": entityType, "entity_id": eventID, "merchid": merchID})
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
	rdx.RdxDel(fmt.Sprintf("merch:%s:%s", eventID, merchID))

	m := mq.Index{EntityType: "merch", EntityId: merchID, Method: "DELETE", ItemType: "event", ItemId: eventID}
	go mq.Emit("merch-deleted", m)

	// // Send response
	// w.WriteHeader(http.StatusOK)
	// w.Write([]byte("Merchandise deleted successfully"))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Merch updated successfully",
	})
}

func BuyMerch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	merchID := ps.ByName("merchid")

	// Retrieve the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

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
	// collection := client.Database("eventdb").Collection("merch")
	var merch structs.Merch // Define the Merch struct based on your schema
	err = db.MerchCollection.FindOne(context.TODO(), bson.M{"entity_id": eventID, "merchid": merchID}).Decode(&merch)
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
	_, err = db.MerchCollection.UpdateOne(context.TODO(), bson.M{"eventid": eventID, "merchid": merchID}, update)
	if err != nil {
		http.Error(w, "Failed to update merch stock", http.StatusInternalServerError)
		return
	}

	userdata.SetUserData("merch", merchID, requestingUserID)

	m := mq.Index{}
	mq.Notify("merch-bought", m)

	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Successfully purchased %d of %s", requestData.Quantity, merch.Name),
	})
}
