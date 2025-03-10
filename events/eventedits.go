package events

import (
	"context"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/mq"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
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

	// Handle banner image upload
	bannerImage, err := handleFileUpload(r, eventID, "banner")
	if err != nil {
		log.Printf("Banner upload failed for event %s: %v", eventID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if bannerImage != "" {
		updateFields["banner_image"] = bannerImage
	}

	// Handle event seating image upload
	seatingPlanImage, err := handleFileUpload(r, eventID, "seating")
	if err != nil {
		log.Printf("Seating Plan upload failed for event %s: %v", eventID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if seatingPlanImage != "" {
		updateFields["event-seating"] = seatingPlanImage
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
	m := mq.Index{EntityType: "event", EntityId: eventID, Action: "PUT"}
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

	m := mq.Index{EntityType: "event", EntityId: eventID, Action: "DELETE"}
	go mq.Emit("event-deleted", m)

	// Send success response
	utils.SendJSONResponse(w, http.StatusOK, map[string]string{"message": "Event deleted successfully"})
}
