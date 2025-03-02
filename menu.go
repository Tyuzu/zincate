package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"naevis/mq"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// Function to handle the creation of menu
func createMenu(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeID := ps.ByName("placeid")
	if placeID == "" {
		http.Error(w, "Place ID is required", http.StatusBadRequest)
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

	// Validate menu data
	if len(name) == 0 || len(name) > 100 {
		http.Error(w, "Name must be between 1 and 100 characters.", http.StatusBadRequest)
		return
	}

	// Create a new Menu instance
	menu := Menu{
		PlaceID:   placeID,
		Name:      name,
		Price:     price,
		Stock:     stock,
		MenuID:    generateID(14), // Generate unique menu ID
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
		savePath := "./menupic/" + menu.MenuID + fileExtension
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
		menu.MenuPhoto = menu.MenuID + fileExtension
	}

	// Insert menu into MongoDB
	// collection := client.Database("placedb").Collection("menu")
	_, err = menuCollection.InsertOne(context.TODO(), menu)
	if err != nil {
		http.Error(w, "Failed to insert menu: "+err.Error(), http.StatusInternalServerError)
		return
	}

	mq.Emit("menu-created")

	// Respond with the created menu
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Menu created successfully.",
		"data":    menu,
	})
}

// Fetch a single menu item
func getMenu(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeID := ps.ByName("placeid")
	menuID := ps.ByName("menuid")
	cacheKey := fmt.Sprintf("menu:%s:%s", placeID, menuID)

	// Check if the menu is cached
	cachedMenu, err := RdxGet(cacheKey)
	if err == nil && cachedMenu != "" {
		// Return cached data
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cachedMenu))
		return
	}

	// collection := client.Database("placedb").Collection("menu")
	var menu Menu
	err = menuCollection.FindOne(context.TODO(), bson.M{"placeid": placeID, "menuid": menuID}).Decode(&menu)
	if err != nil {
		http.Error(w, fmt.Sprintf("Menu not found: %v", err), http.StatusNotFound)
		return
	}

	// Cache the result
	menuJSON, _ := json.Marshal(menu)
	RdxSet(cacheKey, string(menuJSON))

	// Respond with menu data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(menu)
}

// Fetch a list of menu items
func getMenus(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeID := ps.ByName("placeid")
	// cacheKey := fmt.Sprintf("menulist:%s", placeID)
	fmt.Println("::::------------------------------::", placeID)
	// // Check if the menu list is cached
	// cachedMenus, err := RdxGet(cacheKey)
	// if err == nil && cachedMenus != "" {
	// 	// Return cached list
	// 	w.Header().Set("Content-Type", "application/json")
	// 	w.Write([]byte(cachedMenus))
	// 	return
	// }

	// collection := client.Database("placedb").Collection("menu")
	var menuList []Menu
	filter := bson.M{"placeid": placeID}

	cursor, err := menuCollection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Failed to fetch menu", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		var menu Menu
		if err := cursor.Decode(&menu); err != nil {
			http.Error(w, "Failed to decode menu", http.StatusInternalServerError)
			return
		}
		menuList = append(menuList, menu)
	}

	if err := cursor.Err(); err != nil {
		http.Error(w, "Cursor error", http.StatusInternalServerError)
		return
	}

	if len(menuList) == 0 {
		menuList = []Menu{}
	}

	// Cache the list
	// menuListJSON, _ := json.Marshal(menuList)
	// RdxSet(cacheKey, string(menuListJSON))

	// Respond with the list of menu
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(menuList)
}

// Edit a menu item
func editMenu(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeID := ps.ByName("placeid")
	menuID := ps.ByName("menuid")

	// Parse the request body
	var menu Menu
	if err := json.NewDecoder(r.Body).Decode(&menu); err != nil {
		http.Error(w, "Invalid input data", http.StatusBadRequest)
		return
	}

	// Validate menu data
	if menu.Name == "" || menu.Price <= 0 || menu.Stock < 0 {
		http.Error(w, "Invalid menu data: Name, Price, and Stock are required.", http.StatusBadRequest)
		return
	}

	// Prepare update data
	updateFields := bson.M{}
	if menu.Name != "" {
		updateFields["name"] = menu.Name
	}
	if menu.Price > 0 {
		updateFields["price"] = menu.Price
	}
	if menu.Stock >= 0 {
		updateFields["stock"] = menu.Stock
	}

	// Update the menu in MongoDB
	// collection := client.Database("placedb").Collection("menu")
	updateResult, err := menuCollection.UpdateOne(
		context.TODO(),
		bson.M{"placeid": placeID, "menuid": menuID},
		bson.M{"$set": updateFields},
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update menu: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if update was successful
	if updateResult.MatchedCount == 0 {
		http.Error(w, "Menu not found", http.StatusNotFound)
		return
	}

	// Invalidate the specific menu cache
	RdxDel(fmt.Sprintf("menu:%s:%s", placeID, menuID))

	mq.Emit("menu-edited")

	// Send response
	// w.Header().Set("Content-Type", "application/json")
	// w.WriteHeader(http.StatusOK)
	// json.NewEncoder(w).Encode("Menu updated successfully")
	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Menu updated successfully",
	})
}

// Delete a menu item
func deleteMenu(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeID := ps.ByName("placeid")
	menuID := ps.ByName("menuid")

	// Delete the menu from MongoDB
	// collection := client.Database("placedb").Collection("menu")
	deleteResult, err := menuCollection.DeleteOne(context.TODO(), bson.M{"placeid": placeID, "menuid": menuID})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete menu: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if delete was successful
	if deleteResult.DeletedCount == 0 {
		http.Error(w, "Menu not found", http.StatusNotFound)
		return
	}

	// Invalidate the cache
	RdxDel(fmt.Sprintf("menu:%s:%s", placeID, menuID))

	mq.Emit("menu-deleted")

	// // Send response
	// w.WriteHeader(http.StatusOK)
	// w.Write([]byte("Menu deleted successfully"))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Menu updated successfully",
	})
}
