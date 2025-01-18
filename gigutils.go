package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"go.mongodb.org/mongo-driver/bson"
)

// // Utility function to send JSON response
// func sendJSONResponse(w http.ResponseWriter, status int, response interface{}) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(status)
// 	if err := json.NewEncoder(w).Encode(response); err != nil {
// 		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// 	}
// }

// Extract and update gig fields
func updateGigFields(r *http.Request) (bson.M, error) {
	// Parse the multipart form with a 10MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return nil, fmt.Errorf("unable to parse form")
	}

	// Prepare a map for updating fields
	updateFields := bson.M{}

	// Only set the fields that are provided in the form
	if title := r.FormValue("title"); title != "" {
		updateFields["title"] = title
	}

	if date := r.FormValue("date"); date != "" {
		updateFields["date"] = date
	}

	if place := r.FormValue("place"); place != "" {
		updateFields["place"] = place
	}

	if location := r.FormValue("location"); location != "" {
		updateFields["location"] = location
	}

	if description := r.FormValue("description"); description != "" {
		updateFields["description"] = description
	}

	return updateFields, nil
}

// Handle file upload and save banner image if present
func handleGigFileUpload(r *http.Request, gigID string) (string, error) {
	// Handle banner file upload if present
	bannerFile, _, err := r.FormFile("gig-banner")
	if err != nil && err != http.ErrMissingFile {
		return "", fmt.Errorf("error retrieving banner file")
	}
	defer func() {
		if bannerFile != nil {
			bannerFile.Close()
		}
	}()

	// If a new banner is uploaded, save it and return the file path
	if bannerFile != nil {
		// Ensure the directory exists
		if err := os.MkdirAll("./gigpic", os.ModePerm); err != nil {
			return "", fmt.Errorf("error creating directory for banner")
		}

		// Save the banner image
		out, err := os.Create("./gigpic/" + gigID + ".jpg")
		if err != nil {
			return "", fmt.Errorf("error saving banner")
		}
		defer out.Close()

		// Copy the content of the uploaded file to the destination file
		if _, err := io.Copy(out, bannerFile); err != nil {
			return "", fmt.Errorf("error saving banner")
		}

		return gigID + ".jpg", nil
	}

	return "", nil
}

// Validate required fields
func validateGigUpdateFields(updateFields bson.M) error {
	if updateFields["title"] == "" || updateFields["location"] == "" || updateFields["description"] == "" {
		return fmt.Errorf("title, location, and description are required")
	}
	return nil
}

// Delete related data (tickets, media, merch) from collections
func deleteGigRelatedData(gigID string) error {
	// Delete related data from collections
	_, err := client.Database("gigdb").Collection("ticks").DeleteMany(context.TODO(), bson.M{"gigid": gigID})
	if err != nil {
		return fmt.Errorf("error deleting related tickets")
	}

	_, err = client.Database("gigdb").Collection("media").DeleteMany(context.TODO(), bson.M{"gigid": gigID})
	if err != nil {
		return fmt.Errorf("error deleting related media")
	}

	_, err = client.Database("gigdb").Collection("merch").DeleteMany(context.TODO(), bson.M{"gigid": gigID})
	if err != nil {
		return fmt.Errorf("error deleting related merch")
	}

	return nil
}
