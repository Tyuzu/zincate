package events

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/structs"
	"naevis/utils"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetEventsCount(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var ciount int = 3
	utils.SendJSONResponse(w, http.StatusOK, ciount)
}

func GetEvents(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Set the response header to indicate JSON content type
	w.Header().Set("Content-Type", "application/json")

	// Parse pagination query parameters (page and limit)
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	// Default values for pagination
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

	// Calculate skip value based on page and limit
	skip := (page - 1) * limit

	// Convert limit and skip to int64
	int64Limit := int64(limit)
	int64Skip := int64(skip)

	// Get the collection
	// collection := client.Database("eventdb").Collection("events")

	// Create the sort order (descending by createdAt)
	sortOrder := bson.D{{Key: "created_at", Value: -1}}

	// Find events with pagination and sorting
	cursor, err := db.EventsCollection.Find(context.TODO(), bson.M{}, &options.FindOptions{
		Skip:  &int64Skip,
		Limit: &int64Limit,
		Sort:  sortOrder,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var events []structs.Event
	if err = cursor.All(context.TODO(), &events); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	//add eventcount logic here

	// Encode the list of events as JSON and write to the response
	if err := json.NewEncoder(w).Encode(events); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// func GetEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	id := ps.ByName("eventid")

// 	// Aggregation pipeline to fetch event along with related tickets, media, merch, and place name
// 	pipeline := mongo.Pipeline{
// 		bson.D{{Key: "$match", Value: bson.D{{Key: "eventid", Value: id}}}},

// 		// Lookup Tickets
// 		bson.D{{Key: "$lookup", Value: bson.D{
// 			{Key: "from", Value: "ticks"},
// 			{Key: "localField", Value: "eventid"},
// 			{Key: "foreignField", Value: "eventid"},
// 			{Key: "as", Value: "tickets"},
// 		}}},

// 		// Lookup Media
// 		bson.D{{Key: "$lookup", Value: bson.D{
// 			{Key: "from", Value: "media"},
// 			{Key: "let", Value: bson.D{
// 				{Key: "event_id", Value: "$eventid"},
// 			}},
// 			{Key: "pipeline", Value: mongo.Pipeline{
// 				bson.D{{Key: "$match", Value: bson.D{
// 					{Key: "$expr", Value: bson.D{
// 						{Key: "$and", Value: bson.A{
// 							bson.D{{Key: "$eq", Value: bson.A{"$entityid", "$$event_id"}}},
// 							bson.D{{Key: "$eq", Value: bson.A{"$entitytype", "event"}}},
// 						}},
// 					}},
// 				}}},
// 				bson.D{{Key: "$limit", Value: 10}},
// 				bson.D{{Key: "$skip", Value: 0}},
// 			}},
// 			{Key: "as", Value: "media"},
// 		}}},

// 		// Lookup Merch
// 		bson.D{{Key: "$lookup", Value: bson.D{
// 			{Key: "from", Value: "merch"},
// 			{Key: "localField", Value: "eventid"},
// 			{Key: "foreignField", Value: "eventid"},
// 			{Key: "as", Value: "merch"},
// 		}}},

// 		// Lookup Place Name from Places Collection
// 		bson.D{{Key: "$lookup", Value: bson.D{
// 			{Key: "from", Value: "places"},
// 			{Key: "localField", Value: "placeid"},   // Matching placeid in events
// 			{Key: "foreignField", Value: "placeid"}, // Matching placeid in places
// 			{Key: "as", Value: "placeInfo"},
// 		}}},

// 		// Unwind placeInfo to extract the place name (if exists)
// 		bson.D{{Key: "$unwind", Value: bson.D{
// 			{Key: "path", Value: "$placeInfo"},
// 			{Key: "preserveNullAndEmptyArrays", Value: true}, // Allow events with no matching place
// 		}}},

// 		// Add place name field separately
// 		bson.D{{Key: "$addFields", Value: bson.D{
// 			{Key: "placename", Value: "$placeInfo.name"}, // Extract place name
// 		}}},
// 	}

// 	// Execute the aggregation query
// 	cursor, err := db.EventsCollection.Aggregate(context.TODO(), pipeline)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

// 	var event structs.Event
// 	if cursor.Next(context.TODO()) {
// 		if err := cursor.Decode(&event); err != nil {
// 			http.Error(w, "Failed to decode event data", http.StatusInternalServerError)
// 			return
// 		}
// 	} else {
// 		http.Error(w, "Event not found", http.StatusNotFound)
// 		return
// 	}

// 	if event.Tickets == nil {
// 		event.Tickets = []structs.Ticket{}
// 	}
// 	if event.Merch == nil {
// 		event.Merch = []structs.Merch{}
// 	}
// 	if event.FAQs == nil {
// 		event.FAQs = []structs.FAQ{}
// 	}

// 	// Encode the event as JSON and write to response
// 	w.Header().Set("Content-Type", "application/json")
// 	if err := json.NewEncoder(w).Encode(event); err != nil {
// 		http.Error(w, "Failed to encode event data", http.StatusInternalServerError)
// 	}
// }

func GetEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("eventid")

	// Aggregation pipeline to fetch event along with related tickets, media, and merch
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "eventid", Value: id}}}},
		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "ticks"},
			{Key: "localField", Value: "eventid"},
			{Key: "foreignField", Value: "eventid"},
			{Key: "as", Value: "tickets"},
		}}},

		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "media"},
			{Key: "let", Value: bson.D{
				{Key: "event_id", Value: "$eventid"},
			}},
			{Key: "pipeline", Value: mongo.Pipeline{
				bson.D{{Key: "$match", Value: bson.D{
					{Key: "$expr", Value: bson.D{
						{Key: "$and", Value: bson.A{
							bson.D{{Key: "$eq", Value: bson.A{"$entityid", "$$event_id"}}},
							bson.D{{Key: "$eq", Value: bson.A{"$entitytype", "event"}}},
						}},
					}},
				}}},
				bson.D{{Key: "$limit", Value: 10}},
				bson.D{{Key: "$skip", Value: 0}},
			}},
			{Key: "as", Value: "media"},
		}}},

		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "merch"},
			{Key: "let", Value: bson.D{
				{Key: "event_id", Value: "$eventid"},
			}},
			{Key: "pipeline", Value: mongo.Pipeline{
				bson.D{{Key: "$match", Value: bson.D{
					{Key: "$expr", Value: bson.D{
						{Key: "$and", Value: bson.A{
							bson.D{{Key: "$eq", Value: bson.A{"$entity_id", "$$event_id"}}},
							bson.D{{Key: "$eq", Value: bson.A{"$entity_type", "event"}}},
						}},
					}},
				}}},
			}},
			{Key: "as", Value: "merch"},
		}}},
	}

	// Execute the aggregation query
	// db.EventsCollection := client.Database("eventdb").Collection("events")
	cursor, err := db.EventsCollection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var event structs.Event
	if cursor.Next(context.TODO()) {
		if err := cursor.Decode(&event); err != nil {
			http.Error(w, "Failed to decode event data", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	// Encode the event as JSON and write to response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(event); err != nil {
		http.Error(w, "Failed to encode event data", http.StatusInternalServerError)
	}
}

// // FAQ represents a single FAQ structure
// type FAQ struct {
// 	Title   string `json:"title"`
// 	Content string `json:"content"`
// }

func AddFAQs(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	if eventID == "" {
		log.Println("Missing event ID in request")
		http.Error(w, "Missing event ID", http.StatusBadRequest)
		return
	}

	// Parse the request body
	var newFAQ structs.FAQ
	err := json.NewDecoder(r.Body).Decode(&newFAQ)
	if err != nil {
		log.Printf("Invalid request payload: %v", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Validate the input
	if newFAQ.Title == "" || newFAQ.Content == "" {
		http.Error(w, "Title and content are required", http.StatusBadRequest)
		return
	}

	// Append the new FAQ to the FAQs array in MongoDB
	result, err := db.EventsCollection.UpdateOne(
		context.TODO(),
		bson.M{"eventid": eventID},
		bson.M{"$push": bson.M{"faqs": newFAQ}}, // Use $push to append to the array
	)
	if err != nil {
		log.Printf("Error updating event %s: %v", eventID, err)
		http.Error(w, "Error updating event", http.StatusInternalServerError)
		return
	}

	if result.MatchedCount == 0 {
		log.Printf("Event with ID %s not found", eventID)
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	// Respond with success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "FAQ added successfully",
	})
}
