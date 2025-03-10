package events

import (
	"context"
	"fmt"
	"io"
	"naevis/db"
	"naevis/mq"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// Extract and update event fields
func updateEventFields(r *http.Request) (bson.M, error) {
	// Parse the multipart form with a 10MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return nil, fmt.Errorf("unable to parse form: %v", err)
	}

	// Prepare a map for updating fields
	updateFields := bson.M{}

	// Only set the fields that are provided in the form
	if title := r.FormValue("title"); title != "" {
		updateFields["title"] = title
	}

	if dateStr := r.FormValue("date"); dateStr != "" {
		if timeStr := r.FormValue("time"); timeStr != "" {
			// Combine date and time into a single timestamp
			dateTimeStr := fmt.Sprintf("%sT%s", dateStr, timeStr)
			parsedDateTime, err := time.Parse("2006-01-02T15:04:05", dateTimeStr)
			if err != nil {
				return nil, fmt.Errorf("invalid date-time format, expected YYYY-MM-DD and HH:MM:SS")
			}
			updateFields["date"] = parsedDateTime.UTC() // Store as a full UTC timestamp
		} else {
			// Default time to "00:00:00" if not provided
			dateTimeStr := fmt.Sprintf("%sT00:00:00", dateStr)
			parsedDateTime, err := time.Parse("2006-01-02T15:04:05", dateTimeStr)
			if err != nil {
				return nil, fmt.Errorf("invalid date format, expected YYYY-MM-DD")
			}
			updateFields["date"] = parsedDateTime.UTC()
		}
	}

	// if dateStr := r.FormValue("date"); dateStr != "" {
	// 	// Convert date string to time.Time
	// 	parsedDate, err := time.Parse("2006-01-02", dateStr)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("invalid date format, expected YYYY-MM-DD")
	// 	}
	// 	updateFields["date"] = parsedDate.UTC() // Store in UTC format
	// }

	// if timeStr := r.FormValue("time"); timeStr != "" {
	// 	// Parse the time string (expected format: HH:MM:SS)
	// 	parsedTime, err := time.Parse("15:04:05", timeStr)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("invalid time format, expected HH:MM:SS")
	// 	}

	// 	// Store as a string (MongoDB doesn't have a separate time type)
	// 	updateFields["time"] = parsedTime.Format("15:04:05")
	// }

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
		if err := os.MkdirAll("./eventpic", os.ModePerm); err != nil {
			return "", fmt.Errorf("error creating directory for formfile")
		}

		// Save the formfile image
		out, err := os.Create("./eventpic/" + eventID + formfile + ".jpg")
		if err != nil {
			return "", fmt.Errorf("error saving %s", formfile)
		}
		defer out.Close()

		// Copy the content of the uploaded file to the destination file
		if _, err := io.Copy(out, formfileFile); err != nil {
			return "", fmt.Errorf("error saving %s", formfile)
		}

		return eventID + formfile + ".jpg", nil
	}

	m := mq.Index{}
	mq.Notify("event-uploaded", m)

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
