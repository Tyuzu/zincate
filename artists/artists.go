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

	utils.RespondWithJSON(w, http.StatusOK, artists)
}

func GetArtistByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artistId := ps.ByName("id")

	var artist models.Artist
	err := db.ArtistsCollection.FindOne(r.Context(), bson.M{"artistid": artistId}).Decode(&artist)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, artist)
}

func GetArtistsByEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")

	cursor, err := db.ArtistsCollection.Find(r.Context(), bson.M{"events": eventID})
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

	artist := models.Artist{
		ArtistID: utils.GenerateID(12),
		Name:     r.FormValue("name"),
		Bio:      r.FormValue("bio"),
		Category: r.FormValue("category"),
		DOB:      r.FormValue("dob"),
		Place:    r.FormValue("place"),
		Country:  r.FormValue("country"),
		Genres:   parseCSV(r.FormValue("genres")),
		Socials:  parseJSONToMap(r.FormValue("socials")),
		Members:  parseJSONToMembers(r.FormValue("members")),
	}

	if path, err := handleFileUpload(r, "banner", "./static/artistpic/banner"); err == nil {
		artist.Banner = path
	}
	if path, err := handleFileUpload(r, "photo", "./static/artistpic/photo"); err == nil {
		artist.Photo = path
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := db.ArtistsCollection.InsertOne(ctx, artist); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create artist")
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, artist)
}

func UpdateArtist(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	idStr := ps.ByName("id")

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	update := bson.M{}
	fields := map[string]string{
		"name":     r.FormValue("name"),
		"bio":      r.FormValue("bio"),
		"category": r.FormValue("category"),
		"dob":      r.FormValue("dob"),
		"place":    r.FormValue("place"),
		"country":  r.FormValue("country"),
	}

	for k, v := range fields {
		if v != "" {
			update[k] = v
		}
	}

	if genres := parseCSV(r.FormValue("genres")); len(genres) > 0 {
		update["genres"] = genres
	}
	if socials := parseJSONToMap(r.FormValue("socials")); len(socials) > 0 {
		update["socials"] = socials
	}
	if members := parseJSONToMembers(r.FormValue("members")); len(members) > 0 {
		update["members"] = members
	}
	if path, err := handleFileUpload(r, "banner", "./static/artistpic/banner"); err == nil {
		update["banner"] = path
	}
	if path, err := handleFileUpload(r, "photo", "./static/artistpic/photo"); err == nil {
		update["photo"] = path
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := db.ArtistsCollection.UpdateOne(ctx, bson.M{"artistid": idStr}, bson.M{"$set": update})

	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update artist")
		return
	}
	if res.MatchedCount == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Artist updated"})
}

func handleFileUpload(r *http.Request, fieldName, folder string) (string, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return saveFile(file, header, folder)
}

func saveFile(file multipart.File, header *multipart.FileHeader, folder string) (string, error) {
	filename := fmt.Sprintf("%s%s", utils.GenerateID(12), filepath.Ext(header.Filename))
	fullPath := filepath.Join(folder, filename)

	out, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		return "", err
	}

	return filename, nil
}

func parseCSV(input string) []string {
	if input == "" {
		return []string{}
	}
	parts := strings.Split(input, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func parseJSONToMap(input string) map[string]string {
	if input == "" {
		return nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(input), &out); err != nil {
		return map[string]string{"default": input}
	}
	return out
}

func parseJSONToMembers(input string) []models.BandMember {
	if input == "" {
		return nil
	}
	var members []models.BandMember
	if err := json.Unmarshal([]byte(input), &members); err != nil {
		return nil
	}
	return members
}

func DeleteArtistByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artistID := ps.ByName("id")

	if artistID == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "artistID is required")
		return
	}

	filter := bson.M{"artistid": artistID}
	update := bson.M{"deleted": true}

	_, err := db.ArtistsCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete artist")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "Artist deleted successfully"})
}

// package artists

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"mime/multipart"
// 	"net/http"
// 	"os"
// 	"path/filepath"
// 	"strings"
// 	"time"

// 	"naevis/db"
// 	"naevis/models"
// 	"naevis/utils"

// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// )

// func GetAllArtists(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	cursor, err := db.ArtistsCollection.Find(ctx, bson.M{})
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching artists")
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var artists []models.Artist
// 	if err := cursor.All(ctx, &artists); err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Error parsing artist data")
// 		return
// 	}

// 	if len(artists) == 0 {
// 		artists = []models.Artist{}
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, artists)
// }

// func GetArtistByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	artistId := ps.ByName("id")

// 	var artist models.Artist
// 	err := db.ArtistsCollection.FindOne(r.Context(), bson.M{"artistid": artistId}).Decode(&artist)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, artist)
// }

// func GetArtistsByEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	eventIDParam := ps.ByName("eventid")

// 	filter := bson.M{"events": eventIDParam}
// 	cursor, err := db.ArtistsCollection.Find(r.Context(), filter)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching artists")
// 		return
// 	}
// 	defer cursor.Close(r.Context())

// 	var artists []models.Artist
// 	if err := cursor.All(r.Context(), &artists); err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Error decoding artist list")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, artists)
// }

// func CreateArtist(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
// 		return
// 	}

// 	var socials map[string]string
// 	socialsData := r.FormValue("socials")
// 	if socialsData != "" {
// 		if err := json.Unmarshal([]byte(socialsData), &socials); err != nil {
// 			socials = map[string]string{"default": socialsData}
// 		}
// 	}

// 	var members []models.BandMember
// 	membersData := r.FormValue("members")
// 	if membersData != "" {
// 		if err := json.Unmarshal([]byte(membersData), &members); err != nil {
// 			members = []models.BandMember{}
// 		}
// 	}

// 	artist := models.Artist{
// 		ArtistID: utils.GenerateID(12),
// 		Name:     r.FormValue("name"),
// 		Bio:      r.FormValue("bio"),
// 		Category: r.FormValue("category"),
// 		DOB:      r.FormValue("dob"),
// 		Place:    r.FormValue("place"),
// 		Country:  r.FormValue("country"),
// 		Genres:   strings.Split(r.FormValue("genres"), ","),
// 		Socials:  socials,
// 		Members:  members,
// 	}

// 	if bannerFile, bannerHeader, err := r.FormFile("banner"); err == nil {
// 		defer bannerFile.Close()
// 		bannerPath, err := saveFile(bannerFile, bannerHeader, "./static/artistpic/banner")
// 		if err != nil {
// 			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving banner")
// 			return
// 		}
// 		artist.Banner = bannerPath
// 	}

// 	if photoFile, photoHeader, err := r.FormFile("photo"); err == nil {
// 		defer photoFile.Close()
// 		photoPath, err := saveFile(photoFile, photoHeader, "./static/artistpic/photo")
// 		if err != nil {
// 			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving photo")
// 			return
// 		}
// 		artist.Photo = photoPath
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	_, err := db.ArtistsCollection.InsertOne(ctx, artist)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create artist")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusCreated, artist)
// }

// func UpdateArtist(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	idParam := ps.ByName("id")

// 	// Parse the multipart form data (max 10MB)
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

// 	// Handle each form field
// 	for key, value := range formFields {
// 		if value != "" {
// 			switch key {
// 			case "genres":
// 				updateData["genres"] = strings.Split(value, ",")
// 			case "socials":
// 				var socials map[string]string
// 				if err := json.Unmarshal([]byte(value), &socials); err != nil {
// 					// If not JSON, fallback to default key
// 					socials = map[string]string{"default": value}
// 				}
// 				updateData["socials"] = socials
// 			default:
// 				updateData[key] = value
// 			}
// 		}
// 	}

// 	// Handle file uploads

// 	if bannerFile, bannerHeader, err := r.FormFile("banner"); err == nil {
// 		defer bannerFile.Close()
// 		bannerPath, err := saveFile(bannerFile, bannerHeader, "./static/artistpic/banner")
// 		if err != nil {
// 			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving banner")
// 			return
// 		}
// 		updateData["banner"] = bannerPath
// 	}

// 	if photoFile, photoHeader, err := r.FormFile("photo"); err == nil {
// 		defer photoFile.Close()
// 		photoPath, err := saveFile(photoFile, photoHeader, "./static/artistpic/photo")
// 		if err != nil {
// 			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving photo")
// 			return
// 		}
// 		updateData["photo"] = photoPath
// 	}

// 	membersData := r.FormValue("members")
// 	if membersData != "" {
// 		var members []models.BandMember
// 		if err := json.Unmarshal([]byte(membersData), &members); err == nil {
// 			updateData["members"] = members
// 		}
// 	}

// 	update := bson.M{"$set": updateData}

// 	// Update in MongoDB
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	result, err := db.ArtistsCollection.UpdateByID(ctx, idParam, update)
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

// func saveFile(file multipart.File, header *multipart.FileHeader, folder string) (string, error) {
// 	filename := fmt.Sprintf("%s%s", utils.GenerateID(12), filepath.Ext(header.Filename))
// 	filePath := fmt.Sprintf("%s/%s", folder, filename)
// 	fmt.Println("_____________", filePath)
// 	out, err := os.Create(filePath)
// 	if err != nil {
// 		return "", err
// 	}
// 	defer out.Close()

// 	_, err = io.Copy(out, file)
// 	if err != nil {
// 		return "", err
// 	}

// 	return filename, nil
// }
