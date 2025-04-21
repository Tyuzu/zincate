package artists

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"naevis/db"
	"naevis/models"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetAllArtists(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := db.ArtistsCollection.Find(ctx, bson.M{})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching artists")
		return
	}
	defer cursor.Close(ctx)

	var artists []models.Artist
	if err := cursor.All(ctx, &artists); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error parsing artist data")
		return
	}

	if len(artists) == 0 {
		artists = []models.Artist{}
	}

	utils.RespondWithJSON(w, http.StatusOK, artists)
}

func GetArtistByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	idParam := ps.ByName("id")
	objID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid artist ID")
		return
	}

	var artist models.Artist
	err = db.ArtistsCollection.FindOne(r.Context(), bson.M{"_id": objID}).Decode(&artist)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, artist)
}

func GetArtistsByEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventIDParam := ps.ByName("eventid")
	eventObjID, err := primitive.ObjectIDFromHex(eventIDParam)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	filter := bson.M{"events": eventObjID}
	cursor, err := db.ArtistsCollection.Find(r.Context(), filter)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching artists")
		return
	}
	defer cursor.Close(r.Context())

	var artists []models.Artist
	if err := cursor.All(r.Context(), &artists); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error decoding artist list")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, artists)
}

func CreateArtist(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	// Optional JSON-style socials parsing
	var socials map[string]string
	socialsData := r.FormValue("socials")
	if socialsData != "" {
		if err := json.Unmarshal([]byte(socialsData), &socials); err != nil {
			socials = map[string]string{"default": socialsData}
		}
	}

	artist := models.Artist{
		ID:       primitive.NewObjectID(),
		Name:     r.FormValue("name"),
		Bio:      r.FormValue("bio"),
		Category: r.FormValue("category"),
		DOB:      r.FormValue("dob"),
		Place:    r.FormValue("place"),
		Country:  r.FormValue("country"),
		Genres:   strings.Split(r.FormValue("genres"), ","),
		Socials:  socials,
	}

	// Handle banner
	if bannerFile, bannerHeader, err := r.FormFile("banner"); err == nil {
		defer bannerFile.Close()
		bannerPath, err := saveFile(bannerFile, bannerHeader, "./static/artistpic/banner")
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving banner")
			return
		}
		artist.Banner = bannerPath
	}

	// Handle photo
	if photoFile, photoHeader, err := r.FormFile("photo"); err == nil {
		defer photoFile.Close()
		photoPath, err := saveFile(photoFile, photoHeader, "./static/artistpic/photo")
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving photo")
			return
		}
		artist.Photo = photoPath
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.ArtistsCollection.InsertOne(ctx, artist)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create artist")
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, artist)
}

// func CreateArtist(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	// Parse the multipart form data
// 	if err := r.ParseMultipartForm(10 << 20); err != nil { // Limit to 10MB
// 		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
// 		return
// 	}

// 	// Parse socials as a map[string]string (formatted as JSON string in the form field)
// 	var socials map[string]string
// 	socialsData := r.FormValue("socials")
// 	if err := json.Unmarshal([]byte(socialsData), &socials); err != nil {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Invalid socials format")
// 		return
// 	}

// 	// Extract artist data from the form fields
// 	artist := models.Artist{
// 		ID:       primitive.NewObjectID(),
// 		Name:     r.FormValue("name"),
// 		Bio:      r.FormValue("bio"),
// 		Category: r.FormValue("category"),
// 		DOB:      r.FormValue("dob"),
// 		Place:    r.FormValue("place"),
// 		Country:  r.FormValue("country"),
// 		Genres:   strings.Split(r.FormValue("genres"), ","),
// 		Socials:  socials,
// 		// Socials: strings.Split(r.FormValue("socials"), ","),
// 	}

// 	// Handle file uploads (e.g., banner and photo)
// 	banner, _, err := r.FormFile("banner")
// 	if err == nil {
// 		defer banner.Close()
// 		// Process or store the banner file as needed here
// 	}

// 	photo, _, err := r.FormFile("photo")
// 	if err == nil {
// 		defer photo.Close()
// 		// Process or store the photo file as needed here
// 	}

// 	if bannerFile, bannerHeader, err := r.FormFile("banner"); err == nil {
// 		defer bannerFile.Close()
// 		bannerPath, err := saveFile(bannerFile, bannerHeader, "uploads/banners")
// 		if err != nil {
// 			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving banner")
// 			return
// 		}
// 		artist.Banner = bannerPath
// 	}

// 	if photoFile, photoHeader, err := r.FormFile("photo"); err == nil {
// 		defer photoFile.Close()
// 		photoPath, err := saveFile(photoFile, photoHeader, "uploads/photos")
// 		if err != nil {
// 			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving photo")
// 			return
// 		}
// 		artist.Photo = photoPath
// 	}

// 	// Connect to the database
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	_, err = db.ArtistsCollection.InsertOne(ctx, artist)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create artist")
// 		return
// 	}
// 	log.Println("created ccc")
// 	utils.RespondWithJSON(w, http.StatusCreated, artist)
// }

func UpdateArtist(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	idParam := ps.ByName("id")
	objID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid artist ID")
		return
	}

	// Parse the multipart form data (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	updateData := bson.M{}
	formFields := map[string]string{
		"category": r.FormValue("category"),
		"name":     r.FormValue("name"),
		"bio":      r.FormValue("bio"),
		"dob":      r.FormValue("dob"),
		"place":    r.FormValue("place"),
		"country":  r.FormValue("country"),
		"genres":   r.FormValue("genres"),
		"socials":  r.FormValue("socials"),
	}

	// Handle each form field
	for key, value := range formFields {
		if value != "" {
			switch key {
			case "genres":
				updateData["genres"] = strings.Split(value, ",")
			case "socials":
				var socials map[string]string
				if err := json.Unmarshal([]byte(value), &socials); err != nil {
					// If not JSON, fallback to default key
					socials = map[string]string{"default": value}
				}
				updateData["socials"] = socials
			default:
				updateData[key] = value
			}
		}
	}

	// Handle file uploads

	if bannerFile, bannerHeader, err := r.FormFile("banner"); err == nil {
		defer bannerFile.Close()
		bannerPath, err := saveFile(bannerFile, bannerHeader, "./static/artistpic/banner")
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving banner")
			return
		}
		updateData["banner"] = bannerPath
	}

	if photoFile, photoHeader, err := r.FormFile("photo"); err == nil {
		defer photoFile.Close()
		photoPath, err := saveFile(photoFile, photoHeader, "./static/artistpic/photo")
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving photo")
			return
		}
		updateData["photo"] = photoPath
	}

	update := bson.M{"$set": updateData}

	// Update in MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ArtistsCollection.UpdateByID(ctx, objID, update)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update artist")
		return
	}

	if result.MatchedCount == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Artist updated"})
}

// func UpdateArtist(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	idParam := ps.ByName("id")
// 	objID, err := primitive.ObjectIDFromHex(idParam)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Invalid artist ID")
// 		return
// 	}

// 	// Parse the multipart form data
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
// 		return
// 	}

// 	updateData := bson.M{}
// 	formFields := map[string]string{
// 		"category": r.FormValue("category"),
// 		"name":     r.FormValue("name"),
// 		"bio":      r.FormValue("bio"),
// 		"dob":      r.FormValue("dob"),
// 		"place":    r.FormValue("place"),
// 		"country":  r.FormValue("country"),
// 		"genres":   r.FormValue("genres"),
// 		"socials":  r.FormValue("socials"),
// 	}

// 	// Add non-empty fields to the update query
// 	for key, value := range formFields {
// 		if value != "" {
// 			if key == "genres" {
// 				updateData[key] = strings.Split(value, ",")
// 			} else if key == "socials" {
// 				var socials map[string]string
// 				if err := json.Unmarshal([]byte(value), &socials); err != nil {
// 					utils.RespondWithError(w, http.StatusBadRequest, "Invalid socials format")
// 					return
// 				}
// 				updateData[key] = socials
// 			} else {
// 				updateData[key] = value
// 			}
// 		}
// 	}

// 	// Handle file updates (e.g., banner and photo)
// 	if banner, _, err := r.FormFile("banner"); err == nil {
// 		defer banner.Close()
// 		// Process or store the new banner as needed
// 		updateData["banner"] = "new_banner_path_or_url"
// 	}

// 	if photo, _, err := r.FormFile("photo"); err == nil {
// 		defer photo.Close()
// 		// Process or store the new photo as needed
// 		updateData["photo"] = "new_photo_path_or_url"
// 	}

// 	update := bson.M{"$set": updateData}

// 	// Connect to the database
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	result, err := db.ArtistsCollection.UpdateByID(ctx, objID, update)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update artist")
// 		return
// 	}

// 	if result.MatchedCount == 0 {
// 		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Artist updated"})
// }

func saveFile(file multipart.File, header *multipart.FileHeader, folder string) (string, error) {
	filename := fmt.Sprintf("%s%s", utils.GenerateID(12), filepath.Ext(header.Filename))
	filePath := fmt.Sprintf("%s/%s", folder, filename)
	fmt.Println("_____________", filePath)
	out, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		return "", err
	}

	return filename, nil
}
