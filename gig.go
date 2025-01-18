package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CreateGig(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Parse the multipart form with a 10MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	var gig Gig

	// Extract gig details from the form
	gig.Name = r.FormValue("gig-name")
	gig.About = r.FormValue("gig-about")
	gig.Place = r.FormValue("gig-place")
	gig.Area = r.FormValue("gig-area")
	gig.Type = r.FormValue("gig-type")
	gig.Category = r.FormValue("category")
	gig.Contact = r.FormValue("gig-contact")
	gig.Discount = r.FormValue("discount")
	gig.WebsiteURL = r.FormValue("website_url")

	// Parse tags (comma-separated) into an array
	if tags := r.FormValue("tags"); tags != "" {
		gig.Tags = strings.Split(tags, ",")
	}

	// Retrieve the requesting user's ID from context
	requestingUserID, ok := r.Context().Value(userIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}
	gig.CreatorID = requestingUserID
	gig.CreatedAt = time.Now()

	// Generate a unique GigID
	gig.GigID = generateID(14)

	// Check for GigID collisions
	exists := gigsCollection.FindOne(context.TODO(), bson.M{"gigid": gig.GigID}).Err()
	if exists == nil {
		http.Error(w, "Gig ID collision, try again", http.StatusInternalServerError)
		return
	}

	// Handle banner image upload (if provided)
	bannerFile, _, err := r.FormFile("gig-banner")
	if err != nil && err != http.ErrMissingFile {
		http.Error(w, "Error retrieving banner file", http.StatusBadRequest)
		return
	}

	if bannerFile != nil {
		defer bannerFile.Close()
		if err := processBannerFile(bannerFile, gig.GigID, &gig.BannerImage); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Insert gig into MongoDB
	if _, err := gigsCollection.InsertOne(context.TODO(), gig); err != nil {
		log.Printf("Error inserting gig into MongoDB: %v", err)
		http.Error(w, "Error saving gig", http.StatusInternalServerError)
		return
	}

	// Respond with created gig
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(gig); err != nil {
		log.Printf("Error encoding gig response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// EditGig handles updating an existing gig.
func EditGig(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	gigID := ps.ByName("gigid")
	if gigID == "" {
		http.Error(w, "Missing gig ID", http.StatusBadRequest)
		return
	}

	// Extract and validate update fields
	updateFields, err := updateGigFields(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateGigUpdateFields(updateFields); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Handle banner image upload (if any)
	bannerImage, err := handleGigFileUpload(r, gigID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if bannerImage != "" {
		updateFields["banner_image"] = bannerImage
	}

	// Add updated timestamp
	updateFields["updated_at"] = time.Now()

	// Update the gig in MongoDB
	result, err := gigsCollection.UpdateOne(
		context.TODO(),
		bson.M{"gigid": gigID},
		bson.M{"$set": updateFields},
	)
	if err != nil {
		log.Printf("Error updating gig %s: %v", gigID, err)
		http.Error(w, "Error updating gig", http.StatusInternalServerError)
		return
	}

	if result.MatchedCount == 0 {
		http.Error(w, "Gig not found", http.StatusNotFound)
		return
	}

	// Retrieve the updated gig
	var updatedGig Gig
	if err := gigsCollection.FindOne(context.TODO(), bson.M{"gigid": gigID}).Decode(&updatedGig); err != nil {
		log.Printf("Error retrieving updated gig %s: %v", gigID, err)
		http.Error(w, "Error retrieving updated gig", http.StatusInternalServerError)
		return
	}

	// Respond with updated gig
	sendJSONResponse(w, http.StatusOK, updatedGig)
}

// processBannerFile validates and saves the banner file.
func processBannerFile(bannerFile multipart.File, gigID string, bannerPath *string) error {
	// Validate file type
	buff := make([]byte, 512)
	if _, err := bannerFile.Read(buff); err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}
	contentType := http.DetectContentType(buff)
	if !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("invalid file type")
	}
	bannerFile.Seek(0, io.SeekStart) // Reset file pointer

	// Ensure directory exists
	if err := os.MkdirAll("./gigpic", 0755); err != nil {
		return fmt.Errorf("error creating directory for banner")
	}

	// Save the banner image
	sanitizedFileName := filepath.Join("./gigpic", filepath.Base(gigID+".jpg"))
	out, err := os.Create(sanitizedFileName)
	if err != nil {
		return fmt.Errorf("error saving banner")
	}
	defer out.Close()

	if _, err := io.Copy(out, bannerFile); err != nil {
		return fmt.Errorf("error saving banner")
	}

	*bannerPath = filepath.Base(sanitizedFileName)
	return nil
}

func GetGigs(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Set the response header to indicate JSON content type
	w.Header().Set("Content-Type", "application/json")

	// Parse pagination query parameters (page and limit)
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	// Default values for pagination
	page := 1
	limit := 10

	// Parse page and limit, using defaults if invalid
	if pageStr != "" {
		if parsedPage, err := strconv.Atoi(pageStr); err == nil {
			page = parsedPage
		}
	}

	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	// Calculate skip value based on page and limit
	skip := (page - 1) * limit

	// Convert limit and skip to int64
	int64Limit := int64(limit)
	int64Skip := int64(skip)

	// Get the collection
	// collection := client.Database("gigdb").Collection("gigs")

	// Create the sort order (descending by createdAt)
	sortOrder := bson.D{{Key: "created_at", Value: -1}}

	// Find gigs with pagination and sorting
	cursor, err := gigsCollection.Find(context.TODO(), bson.M{}, &options.FindOptions{
		Skip:  &int64Skip,
		Limit: &int64Limit,
		Sort:  sortOrder,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var gigs []Gig
	if err = cursor.All(context.TODO(), &gigs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(gigs) == 0 {
		gigs = []Gig{}
	}
	// Encode the list of gigs as JSON and write to the response
	if err := json.NewEncoder(w).Encode(gigs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func GetGig(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("gigid")

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "gigid", Value: id}}}},
	}

	// Execute the aggregation query
	// gigsCollection := client.Database("gigdb").Collection("gigs")
	cursor, err := gigsCollection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var gig Gig
	if cursor.Next(context.TODO()) {
		if err := cursor.Decode(&gig); err != nil {
			http.Error(w, "Failed to decode gig data", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Gig not found", http.StatusNotFound)
		return
	}

	// Encode the gig as JSON and write to response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(gig); err != nil {
		http.Error(w, "Failed to encode gig data", http.StatusInternalServerError)
	}
}

// Handle deleting gig
func DeleteGig(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	gigID := ps.ByName("gigid")

	// Get the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(userIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	// Get the gig details to verify the creator
	// collection := client.Database("gigdb").Collection("gigs")
	var gig Gig
	err := gigsCollection.FindOne(context.TODO(), bson.M{"gigid": gigID}).Decode(&gig)
	if err != nil {
		http.Error(w, "Gig not found", http.StatusNotFound)
		return
	}

	// Check if the requesting user is the creator of the gig
	if gig.CreatorID != requestingUserID {
		log.Printf("User %s attempted to delete an gig they did not create. GigID: %s", requestingUserID, gigID)
		http.Error(w, "Unauthorized to delete this gig", http.StatusForbidden)
		return
	}

	// Delete the gig from MongoDB
	_, err = gigsCollection.DeleteOne(context.TODO(), bson.M{"gigid": gigID})
	if err != nil {
		http.Error(w, "error deleting gig", http.StatusInternalServerError)
		return
	}

	if err := deleteGigRelatedData(gigID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send success response
	sendJSONResponse(w, http.StatusOK, map[string]string{"message": "Gig deleted successfully"})
}
