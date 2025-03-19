package reviews

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/mq"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// var db.ReviewsCollection *mongo.Collection

// GET /api/reviews/:entityType/:entityId
func GetReviews(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entityType")
	entityId := ps.ByName("entityId")

	skip, limit, filters, sort := parseQueryParams(r)
	filters["entity_type"] = entityType
	filters["entity_id"] = entityId

	// Create options for the Find query
	findOptions := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(sort)

	cursor, err := db.ReviewsCollection.Find(context.TODO(), filters, findOptions)
	if err != nil {
		log.Printf("Error retrieving reviews: %v", err)
		http.Error(w, "Failed to retrieve reviews", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var reviews []structs.Review
	if err = cursor.All(context.TODO(), &reviews); err != nil {
		log.Printf("Error decoding reviews: %v", err)
		http.Error(w, "Failed to retrieve reviews", http.StatusInternalServerError)
		return
	}
	if len(reviews) == 0 {
		reviews = []structs.Review{}
	}
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{
		"status":  http.StatusOK,
		"ok":      true,
		"reviews": reviews,
	}
	log.Println("gets reviews : ", reviews)
	json.NewEncoder(w).Encode(response)
}

// GET /api/reviews/:entityType/:entityId/:reviewId
func GetReview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	reviewId := ps.ByName("reviewId")

	var review structs.Review
	err := db.ReviewsCollection.FindOne(context.TODO(), bson.M{"reviewid": reviewId}).Decode(&review)
	if err != nil {
		http.Error(w, fmt.Sprintf("Review not found: %v", err), http.StatusNotFound)
		return
	}
	log.Println("get review : ", review)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(review)
}

// POST /api/reviews/:entityType/:entityId
func AddReview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userId, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok || userId == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var err error

	entityType := ps.ByName("entityType")
	entityId := ps.ByName("entityId")

	count, err := db.ReviewsCollection.CountDocuments(context.TODO(), bson.M{
		"userId":     userId,
		"entityType": entityType,
		"entityId":   entityId,
	})
	if err != nil {
		log.Printf("Error checking for existing review: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if count > 0 {
		http.Error(w, "You have already reviewed this entity", http.StatusConflict)
		return
	}

	var review structs.Review
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil || review.Rating < 1 || review.Rating > 5 || review.Comment == "" {
		http.Error(w, "Invalid review data", http.StatusBadRequest)
		return
	}

	review.ReviewID = utils.GenerateID(16)
	review.UserID = userId
	review.EntityType = entityType
	review.EntityID = entityId
	// review.Date = time.Now().Format(time.RFC3339)
	review.Date = time.Now()

	inserted, err := db.ReviewsCollection.InsertOne(context.TODO(), review)
	if err != nil {
		http.Error(w, "Failed to insert review: "+err.Error(), http.StatusInternalServerError)
		return
	}

	userdata.SetUserData("review", review.ReviewID, userId)

	m := mq.Index{EntityType: "review", EntityId: review.ReviewID, Method: "POST", ItemId: entityId, ItemType: entityType}
	go mq.Emit("review-added", m)

	log.Println("review : ", review.ReviewID)
	log.Println("inserted review : ", inserted)

	w.WriteHeader(http.StatusCreated)
}

// PUT /api/reviews/:entityType/:entityId/:reviewId
func EditReview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userId, _ := r.Context().Value(globals.UserIDKey).(string)
	reviewId := ps.ByName("reviewId")

	var review structs.Review
	err := db.ReviewsCollection.FindOne(context.TODO(), bson.M{"reviewid": reviewId}).Decode(&review)
	if err != nil {
		http.Error(w, fmt.Sprintf("Review not found: %v", err), http.StatusNotFound)
		return
	}

	if review.UserID != userId && !isAdmin(r.Context()) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var updatedFields map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updatedFields); err != nil {
		http.Error(w, "Invalid update data", http.StatusBadRequest)
		return
	}

	_, err = db.ReviewsCollection.UpdateOne(
		context.TODO(),
		bson.M{"reviewid": reviewId},
		bson.M{"$set": updatedFields},
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update Review: %v", err), http.StatusInternalServerError)
		return
	}

	m := mq.Index{EntityType: "review", EntityId: reviewId, Method: "PUT", ItemId: review.EntityID, ItemType: review.EntityType}
	go mq.Emit("review-edited", m)

	w.WriteHeader(http.StatusOK)
}

// DELETE /api/reviews/:entityType/:entityId/:reviewId
func DeleteReview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userId, _ := r.Context().Value(globals.UserIDKey).(string)
	reviewId := ps.ByName("reviewId")

	var review structs.Review
	err := db.ReviewsCollection.FindOne(context.TODO(), bson.M{"reviewid": reviewId}).Decode(&review)
	if err != nil {
		http.Error(w, fmt.Sprintf("Review not found: %v", err), http.StatusNotFound)
		return
	}

	if review.UserID != userId && !isAdmin(r.Context()) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	_, err = db.ReviewsCollection.DeleteOne(context.TODO(), bson.M{"reviewid": reviewId})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete review: %v", err), http.StatusInternalServerError)
		return
	}

	userdata.DelUserData("review", reviewId, userId)

	m := mq.Index{EntityType: "review", EntityId: reviewId, Method: "DELETE", ItemId: review.EntityID, ItemType: review.EntityType}
	go mq.Emit("review-deleted", m)

	w.WriteHeader(http.StatusOK)
}

// Utility functions remain unchanged (e.g., `parseQueryParams`, `isAdmin`)

// Parse pagination and sorting parameters
func parseQueryParams(r *http.Request) (int64, int64, bson.M, bson.D) {
	query := r.URL.Query()

	page, err := strconv.Atoi(query.Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil || limit < 1 {
		limit = 10
	}

	skip := int64((page - 1) * limit)
	filters := bson.M{}
	if rating := query.Get("rating"); rating != "" {
		ratingVal, _ := strconv.Atoi(rating)
		filters["rating"] = ratingVal
	}

	sort := bson.D{}
	switch query.Get("sort") {
	case "date_asc":
		sort = bson.D{{Key: "date", Value: 1}}
	case "date_desc":
		sort = bson.D{{Key: "date", Value: -1}}
	}

	return skip, int64(limit), filters, sort
}

func isAdmin(ctx context.Context) bool {
	role, ok := ctx.Value(roleKey).(string)
	return ok && role == "admin"
}

const roleKey = "role"
