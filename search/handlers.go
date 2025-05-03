package search

import (
	"encoding/json"
	"log"
	"net/http"

	"naevis/db"
	"naevis/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ctx = db.CTX

// GetResultsByTypeHandler handles requests to /{ENTITY_TYPE}?query=QUERY
func GetResultsByTypeHandler(w http.ResponseWriter, r *http.Request, entityType string, query string) {
	log.Println("Entity Type:", entityType)

	if r.Method != http.MethodGet {
		http.Error(w, "Only GET requests allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get results based on the reverse index
	results := GetResultsOfType(entityType, query)

	// Convert results slice (or map for "all") to JSON and send response
	response, err := json.Marshal(results)
	if err != nil {
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

// GetResultsOfType fetches results based on entity type using a reverse index.
// Instead of mapping to a generic result, we now decode directly into the Event or Place models.
func GetResultsOfType(entityType string, query string) interface{} {
	switch entityType {
	case "events":
		eventIDs, _ := GetIndexResults(entityType, query)
		events := []models.Event{}
		for _, id := range eventIDs {
			filter := bson.M{"eventid": id}
			var ev models.Event
			err := FetchAndDecode(entityType, filter, &ev)
			if err != nil {
				log.Println("Error fetching event:", err)
				continue
			}
			events = append(events, ev)
		}
		log.Println("888888 ", events)
		return events

	case "places":
		placeIDs, _ := GetIndexResults(entityType, query)
		places := []models.Place{}
		for _, id := range placeIDs {
			filter := bson.M{"placeid": id}
			var p models.Place
			err := FetchAndDecode(entityType, filter, &p)
			if err != nil {
				log.Println("Error fetching place:", err)
				continue
			}
			places = append(places, p)
		}
		return places

	case "all":
		// For the "all" case, you might want to return both events and places.
		eventIDs, _ := GetIndexResults("events", query)
		events := []models.Event{}
		for _, id := range eventIDs {
			filter := bson.M{"eventid": id}
			var ev models.Event
			err := FetchAndDecode("events", filter, &ev)
			if err != nil {
				log.Println("Error fetching event:", err)
				continue
			}
			events = append(events, ev)
		}

		placeIDs, _ := GetIndexResults("places", query)
		places := []models.Place{}
		for _, id := range placeIDs {
			filter := bson.M{"placeid": id}
			var p models.Place
			err := FetchAndDecode("places", filter, &p)
			if err != nil {
				log.Println("Error fetching place:", err)
				continue
			}
			places = append(places, p)
		}
		return map[string]interface{}{
			"events": events,
			"places": places,
		}

	default:
		return nil
	}
}

// FetchAndDecode retrieves a document from MongoDB and decodes it into the provided output struct.
func FetchAndDecode(collectionName string, filter bson.M, out interface{}) error {
	collection := db.Client.Database("eventdb").Collection(collectionName)
	projection, exists := Projections[collectionName]
	if !exists {
		projection = bson.M{}
	}
	opts := options.FindOne().SetProjection(projection)
	return collection.FindOne(ctx, filter, opts).Decode(out)
}
