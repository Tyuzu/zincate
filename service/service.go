package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"naevis/structs"
	"naevis/utils"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"naevis/repository"

	"go.mongodb.org/mongo-driver/bson"
)

type EventService struct {
	EventRepo *repository.EventRepository
}

func (svc *EventService) DeleteEvent(ctx context.Context, eventID, requestingUserID string) error {
	// Fetch the event
	event, err := svc.EventRepo.FindEventByID(ctx, eventID)
	if err != nil {
		return errors.New("failed to fetch event: " + err.Error())
	}
	if event == nil {
		return errors.New("event not found")
	}

	// Check if the requesting user is authorized to delete the event
	if event.CreatorID != requestingUserID {
		log.Printf("User %s attempted to delete event %s they did not create", requestingUserID, eventID)
		return errors.New("unauthorized to delete this event")
	}

	// Delete the event and related data
	if err := svc.EventRepo.DeleteEvent(ctx, eventID); err != nil {
		return errors.New("failed to delete event: " + err.Error())
	}
	if err := svc.EventRepo.DeleteRelatedData(ctx, eventID); err != nil {
		return errors.New("failed to delete related data: " + err.Error())
	}

	return nil
}
func NewEventService(repo *repository.EventRepository) *EventService {
	return &EventService{EventRepo: repo}
}

func (svc *EventService) GetEvent(w http.ResponseWriter, r *http.Request, eventID string) {
	ctx := r.Context()

	cursor, err := svc.EventRepo.GetEventWithDetails(ctx, eventID)
	if err != nil {
		http.Error(w, "Failed to fetch event data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var event structs.Event // Replace `interface{}` with your `Event` struct
	if cursor.Next(ctx) {
		if err := cursor.Decode(&event); err != nil {
			http.Error(w, "Failed to decode event data", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(event); err != nil {
		http.Error(w, "Failed to encode event data", http.StatusInternalServerError)
	}
}

func (svc *EventService) CreateEvent(ctx context.Context, event structs.Event, userID string, bannerFile multipart.File) (structs.Event, error) {
	// Populate event data
	event.CreatorID = userID
	event.CreatedAt = time.Now()
	event.EventID = utils.GenerateID(14)

	// Check for EventID collisions
	exists, err := svc.EventRepo.IsEventIDExists(ctx, event.EventID)
	if err != nil {
		return structs.Event{}, errors.New("failed to check event ID: " + err.Error())
	}
	if exists {
		return structs.Event{}, errors.New("event ID collision, try again")
	}

	// Handle the banner image upload
	if bannerFile != nil {
		defer bannerFile.Close()

		// Validate file type
		buff := make([]byte, 512)
		if _, err := bannerFile.Read(buff); err != nil {
			return structs.Event{}, errors.New("error reading banner file")
		}
		contentType := http.DetectContentType(buff)
		if !strings.HasPrefix(contentType, "image/") {
			return structs.Event{}, errors.New("invalid file type for banner")
		}
		bannerFile.Seek(0, io.SeekStart) // Reset file pointer

		// Ensure the directory exists
		if err := os.MkdirAll("./eventpic", 0755); err != nil {
			return structs.Event{}, errors.New("error creating directory for banner")
		}

		// Save the banner image
		sanitizedFileName := filepath.Join("./eventpic", filepath.Base(event.EventID+".jpg"))
		out, err := os.Create(sanitizedFileName)
		if err != nil {
			return structs.Event{}, errors.New("error saving banner")
		}
		defer out.Close()

		if _, err := io.Copy(out, bannerFile); err != nil {
			return structs.Event{}, errors.New("error saving banner")
		}

		// Set the event's banner image field with the saved image path
		event.BannerImage = filepath.Base(sanitizedFileName)
	}

	// Insert event into database
	if err := svc.EventRepo.InsertEvent(ctx, event); err != nil {
		log.Printf("Error inserting event into MongoDB: %v", err)
		return structs.Event{}, errors.New("error saving event")
	}

	return event, nil
}
func (svc *EventService) EditEvent(ctx context.Context, eventID string, updateFields map[string]interface{}, bannerImage string) (structs.Event, error) {
	// Add banner image to the update fields if provided
	if bannerImage != "" {
		updateFields["banner_image"] = bannerImage
	}

	// Add the updated timestamp
	updateFields["updated_at"] = time.Now()

	// Update the event in the repository
	matchedCount, err := svc.EventRepo.UpdateEvent(ctx, eventID, updateFields)
	if err != nil {
		return structs.Event{}, errors.New("failed to update event: " + err.Error())
	}
	if matchedCount == 0 {
		return structs.Event{}, errors.New("event not found")
	}

	// Retrieve the updated event
	updatedEvent, err := svc.EventRepo.FindEventByID(ctx, eventID)
	if err != nil {
		return structs.Event{}, errors.New("failed to retrieve updated event: " + err.Error())
	}

	// Dereference the pointer to return a value
	return *updatedEvent, nil
}

func (svc *EventService) GetEvents(ctx context.Context, page, limit int) ([]structs.Event, error) {
	// Calculate skip value
	skip := int64((page - 1) * limit)
	int64Limit := int64(limit)

	// Sort events by created_at in descending order
	sortOrder := bson.D{{Key: "created_at", Value: -1}}

	// Retrieve events from the repository
	events, err := svc.EventRepo.FindEvents(ctx, skip, int64Limit, sortOrder)
	if err != nil {
		return nil, errors.New("failed to retrieve events: " + err.Error())
	}

	return events, nil
}
