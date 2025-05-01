package artists

import (
	"naevis/media"
	"naevis/merch"
	"naevis/utils"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// func PostNewSong(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	artistID := ps.ByName("id")

// 	err := r.ParseMultipartForm(10 << 20)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Invalid multipart form")
// 		return
// 	}

// 	title := r.FormValue("title")
// 	genre := r.FormValue("genre")
// 	duration := r.FormValue("duration")

// 	file, header, err := r.FormFile("audio")
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Audio file is required")
// 		return
// 	}
// 	defer file.Close()

// 	fileExt := filepath.Ext(header.Filename)
// 	fileName := fmt.Sprintf("%s%s", utils.GenerateID(12), fileExt)
// 	uploadPath := filepath.Join("static", "artistpic", fileName)

// 	outFile, err := os.Create(uploadPath)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save audio")
// 		return
// 	}
// 	defer outFile.Close()
// 	_, err = io.Copy(outFile, file)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Error saving file")
// 		return
// 	}

// 	// audioURL := fmt.Sprintf("artistpic/%s", fileName)

// 	newSong := bson.M{
// 		"title":     title,
// 		"genre":     genre,
// 		"duration":  duration,
// 		"audioUrl":  fileName,
// 		"published": true,
// 	}

// 	update := bson.M{
// 		"$push": bson.M{
// 			"songs": newSong,
// 		},
// 	}

// 	_, err = db.ArtistsCollection.UpdateByID(context.TODO(), artistID, update)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Could not update artist with new song")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusCreated, newSong)
// }

// func DeleteSong(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	artistID := ps.ByName("id")
// 	songID := r.URL.Query().Get("songId") // Again using query param

// 	// First pull the song
// 	filter := bson.M{"artistid": artistID}
// 	update := bson.M{
// 		"$pull": bson.M{
// 			"songs": bson.M{"audioUrl": songID},
// 		},
// 	}

// 	_, err := db.ArtistsCollection.UpdateOne(context.TODO(), filter, update)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete song")
// 		return
// 	}

// 	// Optionally: Delete audio file
// 	audioPath := filepath.Join("static", "artistpic", songID)
// 	err = os.Remove(audioPath)
// 	if err != nil && !os.IsNotExist(err) {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete audio file")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "Song deleted successfully"})
// }

// func EditSong(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	artistID := ps.ByName("id")
// 	songID := r.URL.Query().Get("songId") // Get song id from query param

// 	err := r.ParseMultipartForm(10 << 20)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Invalid multipart form")
// 		return
// 	}

// 	title := r.FormValue("title")
// 	genre := r.FormValue("genre")
// 	duration := r.FormValue("duration")

// 	updateFields := bson.M{}
// 	if title != "" {
// 		updateFields["songs.$.title"] = title
// 	}
// 	if genre != "" {
// 		updateFields["songs.$.genre"] = genre
// 	}
// 	if duration != "" {
// 		updateFields["songs.$.duration"] = duration
// 	}

// 	// Handle optional new audio file
// 	file, header, err := r.FormFile("audio")
// 	if err == nil {
// 		defer file.Close()

// 		fileExt := filepath.Ext(header.Filename)
// 		fileName := fmt.Sprintf("%s%s", utils.GenerateID(12), fileExt)
// 		uploadPath := filepath.Join("static", "artistpic", fileName)

// 		outFile, err := os.Create(uploadPath)
// 		if err != nil {
// 			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save new audio")
// 			return
// 		}
// 		defer outFile.Close()

// 		_, err = io.Copy(outFile, file)
// 		if err != nil {
// 			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving new file")
// 			return
// 		}

// 		updateFields["songs.$.audioUrl"] = fileName
// 	}

// 	filter := bson.M{
// 		"artistid":       artistID,
// 		"songs.audioUrl": songID, // Matching by audio filename as ID
// 	}

// 	update := bson.M{
// 		"$set": updateFields,
// 	}

// 	result, err := db.ArtistsCollection.UpdateOne(context.TODO(), filter, update)
// 	if err != nil || result.MatchedCount == 0 {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update song")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "Song updated successfully"})
// }

// func GetArtistsSongs(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	artistID := ps.ByName("id")

// 	var result struct {
// 		Songs []Song `bson:"songs" json:"songs"`
// 	}

// 	err := db.ArtistsCollection.FindOne(context.TODO(), bson.M{"artistid": artistID}).Decode(&result)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
// 		return
// 	}

// 	// Optional: filter unpublished songs unless authorized
// 	filtered := []Song{}
// 	for _, song := range result.Songs {
// 		if song.Published {
// 			filtered = append(filtered, song)
// 		}
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, filtered)
// }

func GetArtistsAlbums(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	albums := []Album{
		{Title: "Nightfall", ReleaseDate: "2023-10-01", Description: "A journey through dusk.", Published: true},
		{Title: "Unreleased Gems", ReleaseDate: "2025-01-01", Description: "Upcoming exclusives.", Published: false},
	}
	utils.RespondWithJSON(w, http.StatusOK, albums)
}

func GetArtistsPosts(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pr := httprouter.Param{Key: "entityType", Value: "artist"}
	px := httprouter.Param{Key: "eventid", Value: ps.ByName("id")}
	ps = append(ps, pr, px)
	media.GetMedias(w, r, ps)
}

func GetArtistsMerch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	pr := httprouter.Param{Key: "entityType", Value: "artist"}
	px := httprouter.Param{Key: "eventid", Value: ps.ByName("id")}
	ps = append(ps, pr, px)
	merch.GetMerchs(w, r, ps)
}

func GetArtistsevents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	events := []Event{
		{Title: "Summer Fest 2025", Date: "2025-06-15", Venue: "Sunset Arena", City: "Los Angeles", Country: "USA", TicketURL: "http://localhost:5173/event/4s89t5jt6754djt"},
		{Title: "Berlin Beats", Date: "2025-07-20", Venue: "Techno Temple", City: "Berlin", Country: "Germany"},
	}
	utils.RespondWithJSON(w, http.StatusOK, events)
}
