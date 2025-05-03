package artists

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"naevis/db"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SaveUploadedFile saves a multipart file to disk and returns the filename.
func SaveUploadedFile(file multipart.File, header *multipart.FileHeader, destFolder string) (string, error) {
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	fileName := fmt.Sprintf("%s%s", utils.GenerateID(12), ext)
	path := filepath.Join(destFolder, fileName)

	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		return "", err
	}
	return fileName, nil
}

// PostNewSong handles uploading a new song (audio + optional poster).
func PostNewSong(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artistID := ps.ByName("id")

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid multipart form")
		return
	}

	// audio upload (required)
	audioFile, audioHeader, err := r.FormFile("audio")
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Audio file is required")
		return
	}
	audioFileName, err := SaveUploadedFile(audioFile, audioHeader, "static/artistpic/songs")
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save audio file")
		return
	}

	// poster upload (optional)
	var posterFileName string
	if posterFile, posterHeader, err := r.FormFile("poster"); err == nil {
		posterFileName, err = SaveUploadedFile(posterFile, posterHeader, "static/artistpic/posters")
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save poster image")
			return
		}
	}

	// assemble new song
	newSong := Song{
		SongID:      primitive.NewObjectID().Hex(),
		ArtistID:    artistID,
		Title:       r.FormValue("title"),
		Genre:       r.FormValue("genre"),
		Duration:    r.FormValue("duration"),
		Description: r.FormValue("description"),
		AudioURL:    audioFileName,
		Poster:      posterFileName,
		Published:   true,
		Plays:       0,
		UploadedAt:  primitive.NewDateTimeFromTime(time.Now()),
	}

	// push into artist document using a filter on artistid
	filter := bson.M{"artistid": artistID}
	update := bson.M{"$push": bson.M{"songs": newSong}}

	_, err = db.SongsCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to add song to artist")
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, newSong)
}

// DeleteSong removes a song by its songid.
func DeleteSong(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artistID := ps.ByName("id")
	songID := ps.ByName("songId")
	// songID := r.URL.Query().Get("songId")

	if songID == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "songId is required")
		return
	}

	filter := bson.M{"artistid": artistID}
	update := bson.M{"$pull": bson.M{"songs": bson.M{"songid": songID}}}

	_, err := db.SongsCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete song")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "Song deleted successfully"})
}

// EditSong updates song fields (title/genre/duration/description/audio/poster).
func EditSong(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artistID := ps.ByName("id")
	songID := ps.ByName("songId")

	if songID == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "songId is required")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid form data")
		return
	}

	updateFields := bson.M{}
	if v := r.FormValue("title"); v != "" {
		updateFields["songs.$.title"] = v
	}
	if v := r.FormValue("genre"); v != "" {
		updateFields["songs.$.genre"] = v
	}
	if v := r.FormValue("duration"); v != "" {
		updateFields["songs.$.duration"] = v
	}
	if v := r.FormValue("description"); v != "" {
		updateFields["songs.$.description"] = v
	}

	if file, header, err := r.FormFile("audio"); err == nil {
		if fn, err := SaveUploadedFile(file, header, "static/artistpic/songs"); err == nil {
			updateFields["songs.$.audioUrl"] = fn
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save new audio")
			return
		}
	}

	if file, header, err := r.FormFile("poster"); err == nil {
		if fn, err := SaveUploadedFile(file, header, "static/artistpic/posters"); err == nil {
			updateFields["songs.$.poster"] = fn
		} else {
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save new poster")
			return
		}
	}

	filter := bson.M{"artistid": artistID, "songs.songid": songID}
	update := bson.M{"$set": updateFields}

	res, err := db.SongsCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil || res.MatchedCount == 0 {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update song")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "Song updated successfully"})
}

// GetArtistsSongs returns all published songs for an artist.
// If no songs exist, returns an empty array.
func GetArtistsSongs(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artistID := ps.ByName("id")

	var result struct {
		Songs []Song `bson:"songs"`
	}

	// ignore errors; result.Songs will be nil if no document found
	_ = db.SongsCollection.
		FindOne(context.TODO(), bson.M{"artistid": artistID}).
		Decode(&result)

	filtered := make([]Song, 0, len(result.Songs))
	for _, s := range result.Songs {
		if s.Published {
			filtered = append(filtered, s)
		}
	}

	utils.RespondWithJSON(w, http.StatusOK, filtered)
}
