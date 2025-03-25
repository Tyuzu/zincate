package events

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"naevis/db"
	"naevis/mq"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func updateEventFields(r *http.Request) (bson.M, error) {
	// Parse the multipart form with a 10MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return nil, fmt.Errorf("unable to parse form: %v", err)
	}

	updateFields := bson.M{}

	// Extract "event" field from form-data
	eventJSON := r.FormValue("event")
	if eventJSON == "" {
		return nil, fmt.Errorf("missing event data")
	}

	// Define a struct to parse the JSON
	var eventData struct {
		Title       string `json:"title"`
		Date        string `json:"date"`
		Location    string `json:"location"`
		Place       string `json:"place"`
		Description string `json:"description"`
	}

	// Decode the JSON
	if err := json.Unmarshal([]byte(eventJSON), &eventData); err != nil {
		return nil, fmt.Errorf("invalid JSON format: %v", err)
	}

	// Map the fields to updateFields
	if eventData.Title != "" {
		updateFields["title"] = eventData.Title
	}
	if eventData.Date != "" {
		parsedDateTime, err := time.Parse(time.RFC3339, eventData.Date)
		if err != nil {
			return nil, fmt.Errorf("invalid date format, expected RFC3339 (YYYY-MM-DDTHH:MM:SSZ)")
		}
		updateFields["date"] = parsedDateTime.UTC()
	}
	if eventData.Location != "" {
		updateFields["location"] = eventData.Location
	}
	if eventData.Place != "" {
		updateFields["place"] = eventData.Place
	}
	if eventData.Description != "" {
		updateFields["description"] = eventData.Description
	}

	return updateFields, nil
}

// Handle file upload and save formfile image if present
func handleFileUpload(r *http.Request, eventID string, formfile string) (string, error) {
	// Handle formfile file upload if present
	formfileFile, _, err := r.FormFile("event-" + formfile)
	if err != nil && err != http.ErrMissingFile {
		return "", fmt.Errorf("error retrieving formfile file")
	}
	defer func() {
		if formfileFile != nil {
			formfileFile.Close()
		}
	}()

	// If a new formfile is uploaded, save it and return the file path
	if formfileFile != nil {
		// Ensure the directory exists
		if err := os.MkdirAll(eventpicUploadPath, os.ModePerm); err != nil {
			return "", fmt.Errorf("error creating directory for formfile")
		}

		// Save the formfile image
		// out, err := os.Create(eventpicUploadPath + "/" + eventID + formfile + ".jpg")
		out, err := os.Create(eventpicUploadPath + "/" + eventID + ".jpg")
		if err != nil {
			return "", fmt.Errorf("error saving %s", formfile)
		}
		defer out.Close()

		// Copy the content of the uploaded file to the destination file
		if _, err := io.Copy(out, formfileFile); err != nil {
			return "", fmt.Errorf("error saving %s", formfile)
		}

		m := mq.Index{}
		mq.Notify("event-uploaded", m)

		return eventID + formfile + ".jpg", nil
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

// Delete related data (tickets, media, merch) from collections
func deleteRelatedData(eventID string) error {
	// Delete related data from collections
	_, err := db.Client.Database("eventdb").Collection("ticks").DeleteMany(context.TODO(), bson.M{"eventid": eventID})
	if err != nil {
		return fmt.Errorf("error deleting related tickets")
	}

	_, err = db.Client.Database("eventdb").Collection("media").DeleteMany(context.TODO(), bson.M{"eventid": eventID})
	if err != nil {
		return fmt.Errorf("error deleting related media")
	}

	_, err = db.Client.Database("eventdb").Collection("merch").DeleteMany(context.TODO(), bson.M{"eventid": eventID})
	if err != nil {
		return fmt.Errorf("error deleting related merch")
	}

	return nil
}
