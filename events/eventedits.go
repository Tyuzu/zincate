package events

import (
	"context"
	"io"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/mq"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func EditEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	if eventID == "" {
		http.Error(w, "Missing event ID", http.StatusBadRequest)
		return
	}

	var event structs.Event
	// Extract and validate update fields
	updateFields, err := updateEventFields(r)
	if err != nil {
		log.Printf("Invalid update fields for event %s: %v", eventID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateUpdateFields(updateFields); err != nil {
		log.Printf("Validation failed for event %s: %v", eventID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Handle the banner image upload (if present)
	bannerFile, _, err := r.FormFile("event-banner")
	if err != nil && err != http.ErrMissingFile {
		http.Error(w, "Error retrieving banner file", http.StatusBadRequest)
		return
	}

	if bannerFile != nil {
		defer bannerFile.Close()

		// Validate file type
		buff := make([]byte, 512)
		if _, err := bannerFile.Read(buff); err != nil {
			http.Error(w, "Error reading file", http.StatusInternalServerError)
			return
		}
		contentType := http.DetectContentType(buff)
		if !strings.HasPrefix(contentType, "image/") {
			http.Error(w, "Invalid file type", http.StatusBadRequest)
			return
		}
		bannerFile.Seek(0, io.SeekStart) // Reset the file pointer

		// Ensure the directory exists
		if err := os.MkdirAll(eventpicUploadPath, 0755); err != nil {
			http.Error(w, "Error creating directory for banner", http.StatusInternalServerError)
			return
		}

		// Sanitize and save the banner image
		sanitizedFileName := filepath.Join(eventpicUploadPath, "/banner/", filepath.Base(eventID+".jpg"))
		log.Println("---||---", sanitizedFileName)
		out, err := os.Create(sanitizedFileName)
		if err != nil {
			http.Error(w, "Error saving banner", http.StatusInternalServerError)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, bannerFile); err != nil {
			http.Error(w, "Error saving banner", http.StatusInternalServerError)
			return
		}

		// Set the event's banner image field with the saved image path
		event.BannerImage = filepath.Base(sanitizedFileName)
		thumFile := filepath.Join(eventpicUploadPath, "/banner/")
		utils.CreateThumb(eventID, thumFile, ".jpg", 300, 200)
	}

	if event.BannerImage != "" {
		updateFields["banner_image"] = event.BannerImage
	}

	// Handle event seating image upload

	// Handle the seating image upload (if present)
	seatingPlanFile, _, err := r.FormFile("event-seating")
	if err != nil && err != http.ErrMissingFile {
		http.Error(w, "Error retrieving seating file", http.StatusBadRequest)
		return
	}

	if seatingPlanFile != nil {
		defer seatingPlanFile.Close()

		// Validate file type
		buff := make([]byte, 512)
		if _, err := seatingPlanFile.Read(buff); err != nil {
			http.Error(w, "Error reading file", http.StatusInternalServerError)
			return
		}
		contentType := http.DetectContentType(buff)
		if !strings.HasPrefix(contentType, "image/") {
			http.Error(w, "Invalid file type", http.StatusBadRequest)
			return
		}
		seatingPlanFile.Seek(0, io.SeekStart) // Reset the file pointer

		// Ensure the directory exists
		if err := os.MkdirAll(eventpicUploadPath, 0755); err != nil {
			http.Error(w, "Error creating directory for seating plan", http.StatusInternalServerError)
			return
		}

		// Sanitize and save the seating plan image
		sanitizedFileName := filepath.Join(eventpicUploadPath, "/seating/", filepath.Base(eventID+"seating.jpg"))
		log.Println("||---||---||", sanitizedFileName)
		out, err := os.Create(sanitizedFileName)
		if err != nil {
			http.Error(w, "Error saving seating plan", http.StatusInternalServerError)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, seatingPlanFile); err != nil {
			http.Error(w, "Error saving seating plan", http.StatusInternalServerError)
			return
		}

		// Set the event's seating plan image field with the saved image path
		event.SeatingPlanImage = filepath.Base(sanitizedFileName)
	}

	if event.SeatingPlanImage != "" {
		updateFields["seatingplan"] = event.SeatingPlanImage
	}

	// Add updated timestamp in BSON format
	updateFields["updated_at"] = time.Now()

	// Update the event in MongoDB
	result, err := db.EventsCollection.UpdateOne(
		context.TODO(),
		bson.M{"eventid": eventID},
		bson.M{"$set": updateFields},
	)
	if err != nil {
		log.Printf("Error updating event %s: %v", eventID, err)
		http.Error(w, "Error updating event", http.StatusInternalServerError)
		return
	}

	// Check if event was found and updated
	if result.MatchedCount == 0 {
		log.Printf("Event %s not found for update", eventID)
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	// Retrieve the updated event
	var updatedEvent structs.Event
	if err := db.EventsCollection.FindOne(context.TODO(), bson.M{"eventid": eventID}).Decode(&updatedEvent); err != nil {
		log.Printf("Error retrieving updated event %s: %v", eventID, err)
		http.Error(w, "Error retrieving updated event", http.StatusInternalServerError)
		return
	}

	// Emit event update message
	m := mq.Index{EntityType: "event", EntityId: eventID, Method: "PUT"}
	go mq.Emit("event-updated", m)

	// Respond with the updated event
	utils.SendJSONResponse(w, http.StatusOK, updatedEvent)
}

// Handle deleting event
func DeleteEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")

	// Get the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	// Get the event details to verify the creator
	// collection := client.Database("eventdb").Collection("events")
	var event structs.Event
	err := db.EventsCollection.FindOne(context.TODO(), bson.M{"eventid": eventID}).Decode(&event)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	// Check if the requesting user is the creator of the event
	if event.CreatorID != requestingUserID {
		log.Printf("User %s attempted to delete an event they did not create. structs.EventID: %s", requestingUserID, eventID)
		http.Error(w, "Unauthorized to delete this event", http.StatusForbidden)
		return
	}

	// Delete the event from MongoDB
	_, err = db.EventsCollection.DeleteOne(context.TODO(), bson.M{"eventid": eventID})
	if err != nil {
		http.Error(w, "error deleting event", http.StatusInternalServerError)
		return
	}

	// Delete related data (tickets, media, merch)
	if err := deleteRelatedData(eventID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	userdata.DelUserData("event", event.EventID, requestingUserID)

	m := mq.Index{EntityType: "event", EntityId: eventID, Method: "DELETE"}
	go mq.Emit("event-deleted", m)

	// Send success response
	utils.SendJSONResponse(w, http.StatusOK, map[string]string{"message": "Event deleted successfully"})
}
