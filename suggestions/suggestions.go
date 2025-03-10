package suggestions

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/rdx"
	"naevis/structs"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func SuggestFollowers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	currentUserID := r.URL.Query().Get("userid")
	if currentUserID == "" {
		http.Error(w, "Missing userid", http.StatusBadRequest)
		return
	}

	userID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Pagination parameters
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 {
		limit = 10
	}

	skip := (page - 1) * limit

	// Fetch user's follow data
	var followData structs.UserFollow
	err = db.FollowingsCollection.FindOne(context.TODO(), bson.M{"userid": currentUserID}).Decode(&followData)
	if err != nil && err != mongo.ErrNoDocuments {
		http.Error(w, "Failed to fetch follow data", http.StatusInternalServerError)
		return
	}

	// Exclude already followed users + current user
	excludedUserIDs := append(followData.Follows, currentUserID, userID)

	// Query for suggested users
	filter := bson.M{"userid": bson.M{"$nin": excludedUserIDs}}
	options := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetProjection(bson.M{
			"userid":   1,
			"username": 1,
			"bio":      1,
		})

	cursor, err := db.UserCollection.Find(context.TODO(), filter, options)
	if err != nil {
		http.Error(w, "Failed to fetch suggestions", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	// Collect suggested users
	var suggestedUsers []structs.UserSuggest
	for cursor.Next(context.TODO()) {
		var suggestedUser structs.UserSuggest
		if err := cursor.Decode(&suggestedUser); err == nil {
			// Explicitly set is_following: false
			suggestedUser.IsFollowing = false
			suggestedUsers = append(suggestedUsers, suggestedUser)
		}
	}

	// Handle empty response case
	if len(suggestedUsers) == 0 {
		suggestedUsers = []structs.UserSuggest{}
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(suggestedUsers); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

/***************************************************/
func GetPlaceSuggestions(ctx context.Context, query string) ([]structs.Suggestion, error) {
	var suggestions []structs.Suggestion

	// Use Redis KEYS command to find matching place suggestions by name
	// (this is a simple approach, you may want a more efficient search strategy)
	keys, err := rdx.Conn.Keys(ctx, fmt.Sprintf("suggestions:place:%s*", query)).Result()
	if err != nil {
		return nil, err
	}

	// Retrieve the corresponding place data
	for _, key := range keys {
		var suggestion structs.Suggestion
		err := rdx.Conn.Get(ctx, key).Scan(&suggestion)
		if err != nil {
			return nil, err
		}
		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
}

func SuggestionsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	suggestions, err := GetPlaceSuggestions(ctx, query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching suggestions: %v", err), http.StatusInternalServerError)
		return
	}
	log.Println("handler sugg : ", suggestions)
	if len(suggestions) == 0 {
		suggestions = []structs.Suggestion{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(suggestions)
}

func GetNearbyPlaces(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	curplace := r.URL.Query().Get("place")
	if len(curplace) != 14 {
		fmt.Println("wronggg")
	} else {
		fmt.Println(curplace)
	}
	fmt.Println(r.URL.Query().Get("lng"))
	fmt.Println(r.URL.Query().Get("lng"))

	cursor, err := db.PlacesCollection.Find(context.TODO(), bson.M{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var places []structs.Place
	if err = cursor.All(context.TODO(), &places); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If no places are found, return an empty array
	if places == nil {
		places = []structs.Place{}
	}

	// Create a slice of sanitized places
	var sanitizedPlaces []map[string]any
	for _, place := range places {
		if place.PlaceID == curplace {
			continue
		}
		sanitizedPlaces = append(sanitizedPlaces, map[string]any{
			"placeid":     place.PlaceID,
			"name":        place.Name,
			"category":    place.Category,
			"capacity":    place.Capacity,
			"reviewCount": place.ReviewCount,
		})
	}

	// Encode and return places data
	json.NewEncoder(w).Encode(sanitizedPlaces)
}

// // use this in production
// func getNearbyPlaces(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	w.Header().Set("Content-Type", "application/json")

// 	// Get latitude & longitude from request query
// 	lat, err1 := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
// 	lng, err2 := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
// 	if err1 != nil || err2 != nil {
// 		http.Error(w, "Invalid latitude or longitude", http.StatusBadRequest)
// 		return
// 	}

// 	// Define geospatial query (requires a 2dsphere index)
// 	filter := bson.M{
// 		"location": bson.M{
// 			"$near": bson.M{
// 				"$geometry": bson.M{
// 					"type":        "Point",
// 					"coordinates": []float64{lng, lat}, // MongoDB requires [longitude, latitude]
// 				},
// 				"$maxDistance": 5000, // Max distance in meters (5 km)
// 			},
// 		},
// 	}

// 	// Fetch places from MongoDB
// 	cursor, err := db.PlacesCollection.Find(context.TODO(), filter)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

// 	var places []Place
// 	if err = cursor.All(context.TODO(), &places); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// Return JSON response
// 	json.NewEncoder(w).Encode(places)
// }

// // do not use this. Avoid fetching all data & filtering manually unless you have a tiny dataset.

// // func getNearbyPlaces(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// // 	getNearbyPlacesWithoutIndex(w, r, ps)
// // }

// // func getNearbyPlacesWithoutIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// // 	w.Header().Set("Content-Type", "application/json")

// // 	// Get latitude & longitude from request query
// // 	lat, err1 := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
// // 	lng, err2 := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
// // 	if err1 != nil || err2 != nil {
// // 		http.Error(w, "Invalid latitude or longitude", http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Fetch all places from MongoDB (inefficient)
// // 	cursor, err := db.PlacesCollection.Find(context.TODO(), bson.M{})
// // 	if err != nil {
// // 		http.Error(w, err.Error(), http.StatusInternalServerError)
// // 		return
// // 	}
// // 	defer cursor.Close(context.TODO())

// // 	var allPlaces []NearbyPlace
// // 	if err = cursor.All(context.TODO(), &allPlaces); err != nil {
// // 		http.Error(w, err.Error(), http.StatusInternalServerError)
// // 		return
// // 	}

// // 	// Manually filter places based on distance (inefficient)
// // 	var nearbyPlaces []NearbyPlace
// // 	for _, place := range allPlaces {
// // 		// Ensure location field exists

// // 		placeLng := place.Location.Longitude
// // 		placeLat := place.Location.Latitude

// // 		// Calculate distance (Haversine formula approximation)
// // 		distance := haversine(lat, lng, placeLat, placeLng)
// // 		if distance <= 5.0 { // 5 km threshold
// // 			nearbyPlaces = append(nearbyPlaces, place)
// // 		}
// // 	}

// // 	if len(nearbyPlaces) == 0 {
// // 		nearbyPlaces = []NearbyPlace{}
// // 	}

// // 	// Return JSON response
// // 	json.NewEncoder(w).Encode(nearbyPlaces)
// // }

// // type NearbyPlace struct {
// // 	Location Coordinates
// // 	Name     string
// // 	PlaceID  string
// // 	Category string
// // 	Rating   int
// // 	Distance string
// // }

// // func haversine(lat1, lon1, lat2, lon2 float64) float64 {
// // 	const earthRadius = 6371.0 // Earth's radius in kilometers
// // 	dLat := (lat2 - lat1) * (math.Pi / 180.0)
// // 	dLon := (lon2 - lon1) * (math.Pi / 180.0)

// // 	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
// // 		math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*
// // 			math.Sin(dLon/2)*math.Sin(dLon/2)

// // 	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
// // 	return earthRadius * c // Distance in km
// // }
