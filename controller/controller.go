package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"naevis/constants"
	"naevis/service"
	"naevis/structs"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

type EventController struct {
	EventSvc *service.EventService
}

func NewEventController(eventSvc *service.EventService) *EventController {
	return &EventController{EventSvc: eventSvc}
}

func (ctrl *EventController) GetEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	ctrl.EventSvc.GetEvent(w, r, eventID)
}

func (ctrl *EventController) DeleteEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")

	// Get the requesting user ID from the context
	requestingUserID, ok := r.Context().Value(constants.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	// Call the service to delete the event
	err := ctrl.EventSvc.DeleteEvent(r.Context(), eventID, requestingUserID)
	if err != nil {
		if err.Error() == "event not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else if err.Error() == "unauthorized to delete this event" {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Send success response
	utils.SendJSONResponse(w, http.StatusOK, map[string]string{"message": "Event deleted successfully"})
}

func (ctrl *EventController) CreateEvent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Parse multipart form with a 10MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	var event structs.Event
	if r.FormValue("event") == "" {
		http.Error(w, "Missing event data", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal([]byte(r.FormValue("event")), &event); err != nil {
		http.Error(w, "Invalid event data", http.StatusBadRequest)
		return
	}

	// Get the requesting user ID from the context
	requestingUserID, ok := r.Context().Value(constants.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	// Handle the banner file (if present)
	bannerFile, _, err := r.FormFile("banner")
	if err != nil && err != http.ErrMissingFile {
		http.Error(w, "Error retrieving banner file", http.StatusBadRequest)
		return
	}

	// Call the service to create the event
	createdEvent, err := ctrl.EventSvc.CreateEvent(r.Context(), event, requestingUserID, bannerFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with the created event
	w.WriteHeader(http.StatusCreated) // 201 Created
	if err := json.NewEncoder(w).Encode(createdEvent); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (ctrl *EventController) EditEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	if eventID == "" {
		http.Error(w, "Missing event ID", http.StatusBadRequest)
		return
	}

	// Extract update fields
	updateFields, err := updateEventFields(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate update fields
	if err := validateUpdateFields(updateFields); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Handle banner file upload
	bannerImage, err := handleFileUpload(r, eventID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Call service to edit the event
	updatedEvent, err := ctrl.EventSvc.EditEvent(r.Context(), eventID, updateFields, bannerImage)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with the updated event
	utils.SendJSONResponse(w, http.StatusOK, updatedEvent)
}

func (ctrl *EventController) GetEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Parse pagination query parameters
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	// Default pagination values
	page := 1
	limit := 10

	// Parse page and limit, using defaults if invalid
	if pageStr != "" {
		if parsedPage, err := strconv.Atoi(pageStr); err == nil {
			page = parsedPage
		}
	}
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	// Call service to retrieve events
	events, err := ctrl.EventSvc.GetEvents(r.Context(), page, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with the list of events
	utils.SendJSONResponse(w, http.StatusOK, events)
}

// Extract and update event fields
func updateEventFields(r *http.Request) (bson.M, error) {
	// Parse the multipart form with a 10MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return nil, fmt.Errorf("unable to parse form")
	}

	// Prepare a map for updating fields
	updateFields := bson.M{}

	// Only set the fields that are provided in the form
	if title := r.FormValue("title"); title != "" {
		updateFields["title"] = title
	}

	if date := r.FormValue("date"); date != "" {
		updateFields["date"] = date
	}

	if place := r.FormValue("place"); place != "" {
		updateFields["place"] = place
	}

	if location := r.FormValue("location"); location != "" {
		updateFields["location"] = location
	}

	if description := r.FormValue("description"); description != "" {
		updateFields["description"] = description
	}

	return updateFields, nil
}

// Handle file upload and save banner image if present
func handleFileUpload(r *http.Request, eventID string) (string, error) {
	// Handle banner file upload if present
	bannerFile, _, err := r.FormFile("event-banner")
	if err != nil && err != http.ErrMissingFile {
		return "", fmt.Errorf("error retrieving banner file")
	}
	defer func() {
		if bannerFile != nil {
			bannerFile.Close()
		}
	}()

	// If a new banner is uploaded, save it and return the file path
	if bannerFile != nil {
		// Ensure the directory exists
		if err := os.MkdirAll("./eventpic", os.ModePerm); err != nil {
			return "", fmt.Errorf("error creating directory for banner")
		}

		// Save the banner image
		out, err := os.Create("./eventpic/" + eventID + ".jpg")
		if err != nil {
			return "", fmt.Errorf("error saving banner")
		}
		defer out.Close()

		// Copy the content of the uploaded file to the destination file
		if _, err := io.Copy(out, bannerFile); err != nil {
			return "", fmt.Errorf("error saving banner")
		}

		return eventID + ".jpg", nil
	}

	return "", nil
}

// Validate required fields
func validateUpdateFields(updateFields bson.M) error {
	if updateFields["title"] == "" || updateFields["location"] == "" || updateFields["description"] == "" {
		return fmt.Errorf("title, location, and description are required")
	}
	return nil
}
