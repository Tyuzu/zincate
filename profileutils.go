package main

import (
	"fmt"
	"naevis/mq"
	"net/http"
	"os"

	"github.com/disintegration/imaging"
	"go.mongodb.org/mongo-driver/bson"
)

// createThumbnail creates a thumbnail of the image at inputPath and saves it at outputPath.
func createThumbnail(inputPath, outputPath string, width, height int) error {
	// width := 100
	// height := 100

	img, err := imaging.Open(inputPath)
	if err != nil {
		return err
	}
	resizedImg := imaging.Resize(img, width, height, imaging.Lanczos)

	mq.Emit("thumbnail-created")

	return imaging.Save(resizedImg, outputPath)
}

func uploadBannerHandler(w http.ResponseWriter, r *http.Request, claims *Claims) (bson.M, error) {
	_ = w
	update := bson.M{}
	// Parse form data
	err := r.ParseMultipartForm(10 << 20) // Limit to 10MB
	if err != nil {
		// http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return nil, fmt.Errorf("error parsing form data: %w", err)
	}

	// Retrieve the file
	file, _, err := r.FormFile("banner_picture")
	if err != nil {
		// http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return nil, fmt.Errorf("error retrieving the file: %w", err)
	}
	defer file.Close()

	// Save the file
	// filePath := filepath.Join("./userpic/banner", handler.Filename)
	filePath := "./userpic/banner/" + claims.Username + ".jpg"
	outFile, err := os.Create(filePath)
	if err != nil {
		// http.Error(w, "Error saving the file", http.StatusInternalServerError)
		return nil, fmt.Errorf("error saving the file: %w", err)
	}
	defer outFile.Close()

	// Write the file content
	_, err = outFile.ReadFrom(file)
	if err != nil {
		// http.Error(w, "Error writing the file", http.StatusInternalServerError)
		return nil, fmt.Errorf("error writing the file: %w", err)
	}

	update["banner_picture"] = claims.Username + ".jpg"

	mq.Emit("banner-uploaded")

	return update, nil

	// // Respond with success
	// w.Header().Set("Content-Type", "application/json")
	// w.WriteHeader(http.StatusOK)
	// fmt.Fprintf(w, `{"message": "Thumbnail uploaded successfully", "ok": true, "file": "%s"}`, handler.Filename)
}

func updateProfilePictures(w http.ResponseWriter, r *http.Request, claims *Claims) (bson.M, error) {
	_ = w
	update := bson.M{}
	// Parse form data
	err := r.ParseMultipartForm(10 << 20) // Limit to 10MB
	if err != nil {
		// http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return nil, fmt.Errorf("error parsing form data: %w", err)
	}

	// Retrieve the file
	file, _, err := r.FormFile("avatar_picture")
	if err != nil {
		// http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return nil, fmt.Errorf("error retrieving the file: %w", err)
	}
	defer file.Close()

	// Save the file
	// filePath := filepath.Join("./userpic", handler.Filename)
	filePath := "./userpic/" + claims.Username + ".jpg"
	outFile, err := os.Create(filePath)
	if err != nil {
		// http.Error(w, "Error saving the file", http.StatusInternalServerError)
		return nil, fmt.Errorf("error saving the file: %w", err)
	}
	defer outFile.Close()

	// Write the file content
	_, err = outFile.ReadFrom(file)
	if err != nil {
		// http.Error(w, "Error writing the file", http.StatusInternalServerError)
		return nil, fmt.Errorf("error writing the file: %w", err)
	}
	thumbPath := "./userpic/thumb/" + claims.UserID + ".jpg"
	createThumbnail(filePath, thumbPath, 100, 100)

	update["profile_picture"] = claims.Username + ".jpg"

	mq.Emit("avatar-uploaded")

	return update, nil

	// // Respond with success
	// w.Header().Set("Content-Type", "application/json")
	// w.WriteHeader(http.StatusOK)
	// fmt.Fprintf(w, `{"message": "Thumbnail uploaded successfully", "ok": true, "file": "%s"}`, handler.Filename)
}
