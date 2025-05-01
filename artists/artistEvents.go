package artists

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/mq"
	"naevis/profile"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// ArtistEvent Struct
type ArtistEvent struct {
	EventID   string `bson:"eventid,omitempty" json:"eventid"`
	ArtistID  string `bson:"artistid" json:"artistid"`
	Title     string `bson:"title" json:"title"`
	Date      string `bson:"date" json:"date"`
	Venue     string `bson:"venue" json:"venue"`
	City      string `bson:"city" json:"city"`
	Country   string `bson:"country" json:"country"`
	CreatorID string `bson:"creatorid" json:"creatorid"`
	TicketURL string `bson:"ticket_url,omitempty" json:"ticketUrl,omitempty"`
}

// Get Artist Events
func GetArtistEvents(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artistID := ps.ByName("id")

	var artistevents []ArtistEvent
	cursor, err := db.ArtistEventsCollection.Find(context.TODO(), bson.M{"artistid": artistID})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer cursor.Close(context.TODO())

	if err := cursor.All(context.TODO(), &artistevents); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to parse artistevents")
		return
	}

	if artistevents == nil {
		artistevents = []ArtistEvent{}
	}

	utils.RespondWithJSON(w, http.StatusOK, artistevents)
}

// Create Artist Event
func CreateArtistEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var artistevent ArtistEvent
	if err := json.NewDecoder(r.Body).Decode(&artistevent); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	artistevent.ArtistID = ps.ByName("id")
	artistevent.EventID = utils.GenerateID(14)
	insertResult, err := db.ArtistEventsCollection.InsertOne(context.TODO(), artistevent)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	tokenString := r.Header.Get("Authorization")
	claims, err := profile.ValidateJWT(tokenString)
	if err != nil {
		log.Printf("JWT validation error: %v", err) // Log the error for debugging
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	artistevent.CreatorID = claims.UserID
	addEventToDB(artistevent)

	utils.RespondWithJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "ArtistEvent created successfully",
		"id":      insertResult.InsertedID,
	})
}

// Update Artist Event
func UpdateArtistEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artisteventID := ps.ByName("id")

	var updateData bson.M
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	result, err := db.ArtistEventsCollection.UpdateOne(context.TODO(), bson.M{"eventid": artisteventID}, bson.M{"$set": updateData})
	if err != nil || result.ModifiedCount == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "ArtistEvent not found or update failed")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "ArtistEvent updated successfully"})
}

// Delete Artist Event
func DeleteArtistEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artisteventID := ps.ByName("id")

	result, err := db.ArtistEventsCollection.DeleteOne(context.TODO(), bson.M{"eventid": artisteventID})
	if err != nil || result.DeletedCount == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "ArtistEvent not found or deletion failed")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "ArtistEvent deleted successfully"})
}

func addEventToDB(artistEvent ArtistEvent) (string, error) {
	var event structs.Event

	// dateString := "2025-04-29 10:00:00"
	// layout := "2006-01-02 15:04:05" // Format string must match the input string

	dateString := artistEvent.Date
	layout := "2006-01-02"

	dateToSave, _ := time.Parse(layout, dateString)

	// event.CreatorID = artistEvent.ArtistID
	event.CreatorID = artistEvent.CreatorID
	event.CreatedAt = time.Now().UTC() // ✅ Ensure UTC timestamp
	event.Date = dateToSave.UTC()      // ✅ Force UTC before saving
	event.Status = "active"
	event.FAQs = []structs.FAQ{}
	event.EventID = artistEvent.EventID
	event.Artists = []string{artistEvent.ArtistID}
	event.Title = artistEvent.Title
	event.Location = artistEvent.Venue
	event.Published = "draft"
	event.Category = "concert"

	// Insert the event into MongoDB
	result, err := db.EventsCollection.InsertOne(context.TODO(), event)
	if err != nil || result.InsertedID == nil {
		log.Printf("Error inserting event into MongoDB: %v", err)
		return "", err
	}

	userdata.SetUserData("event", event.EventID, artistEvent.ArtistID)

	// ✅ Emit event for messaging queue (if needed)
	go mq.Emit("event-created", mq.Index{
		EntityType: "event", EntityId: event.EventID, Method: "POST",
	})
	return "", err
}
