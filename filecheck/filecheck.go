package filecheck

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"naevis/db"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type FileMetadata struct {
	ID     primitive.ObjectID `bson:"_id,omitempty"`
	Hash   string             `bson:"hash"`
	PostID string             `bson:"postid"`
	URL    string             `bson:"url"`
	Users  []string           `bson:"users"` // List of user IDs who saved the file
}

func CheckFileExists(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var req struct {
		Hash string `json:"hash"`
	}

	// Parse JSON request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Check in MongoDB
	var file FileMetadata
	err := db.FilesCollection.FindOne(context.TODO(), bson.M{"hash": req.Hash}).Decode(&file)

	if err == mongo.ErrNoDocuments {
		json.NewEncoder(w).Encode(map[string]any{"exists": false})
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// File exists, return its metadata
	json.NewEncoder(w).Encode(map[string]interface{}{
		"exists": true,
		"url":    file.URL,
		"postid": file.PostID,
	})
}

func UploadFile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Parse form data
	err := r.ParseMultipartForm(10 << 20) // 10MB max file size
	if err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Invalid file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Compute SHA-256 hash
	hash := ComputeFileHash(file)
	userID := r.FormValue("userID") // Get user ID from form data

	// Check if the file exists
	var existingFile FileMetadata
	err = db.FilesCollection.FindOne(context.TODO(), bson.M{"hash": hash}).Decode(&existingFile)

	if err == nil {
		// File exists, just add user to the list
		_, err := db.FilesCollection.UpdateOne(context.TODO(),
			bson.M{"hash": hash},
			bson.M{"$addToSet": bson.M{"users": userID}}, // Prevents duplicate user entries
		)
		if err != nil {
			http.Error(w, "Failed to update user list", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{
			"message": "File already exists, linked to user.",
			"url":     existingFile.URL,
		})
		return
	}

	// Save the file if it's new
	filePath := "./static/uploads/" + handler.Filename
	outFile, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()
	io.Copy(outFile, file)

	// Save file metadata in MongoDB
	newFile := FileMetadata{
		Hash:  hash,
		URL:   filePath,
		Users: []string{userID},
	}
	_, err = db.FilesCollection.InsertOne(context.TODO(), newFile)
	if err != nil {
		http.Error(w, "Failed to save file metadata", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"message": "File uploaded successfully.",
		"url":     filePath,
	})
}

func ComputeFileHash(file multipart.File) string {
	hasher := sha256.New()
	file.Seek(0, io.SeekStart) // Reset file pointer before reading
	io.Copy(hasher, file)
	return hex.EncodeToString(hasher.Sum(nil))
}

func RemoveUserFile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var req struct {
		Hash   string `json:"hash"`
		UserID string `json:"userID"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Remove user from file's user list
	result, err := db.FilesCollection.UpdateOne(context.TODO(),
		bson.M{"hash": req.Hash},
		bson.M{"$pull": bson.M{"users": req.UserID}}, // Remove user from the array
	)

	if err != nil || result.MatchedCount == 0 {
		http.Error(w, "File not found or user not linked", http.StatusNotFound)
		return
	}

	// Check if there are no more users left
	var file FileMetadata
	err = db.FilesCollection.FindOne(context.TODO(), bson.M{"hash": req.Hash}).Decode(&file)
	if err == nil && len(file.Users) == 0 {
		// No users left, delete the file from storage and database
		os.Remove(file.URL) // Delete file from disk
		db.FilesCollection.DeleteOne(context.TODO(), bson.M{"hash": req.Hash})
	}

	json.NewEncoder(w).Encode(map[string]string{
		"message": "User removed from file.",
	})
}
