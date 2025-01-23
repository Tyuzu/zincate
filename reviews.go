package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/mq"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// var reviewsCollection *mongo.Collection

// GET /api/reviews/:entityType/:entityId
func getReviews(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	cursor, err := reviewsCollection.Find(context.TODO(), filters, findOptions)
	if err != nil {
		log.Printf("Error retrieving reviews: %v", err)
		http.Error(w, "Failed to retrieve reviews", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var reviews []Review
	if err = cursor.All(context.TODO(), &reviews); err != nil {
		log.Printf("Error decoding reviews: %v", err)
		http.Error(w, "Failed to retrieve reviews", http.StatusInternalServerError)
		return
	}
	if len(reviews) == 0 {
		reviews = []Review{}
	}
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":  http.StatusOK,
		"ok":      true,
		"reviews": reviews,
	}
	log.Println("gets reviews : ", reviews)
	json.NewEncoder(w).Encode(response)
}

// GET /api/reviews/:entityType/:entityId/:reviewId
func getReview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	reviewId := ps.ByName("reviewId")

	var review Review
	err := reviewsCollection.FindOne(context.TODO(), bson.M{"reviewid": reviewId}).Decode(&review)
	if err != nil {
		http.Error(w, fmt.Sprintf("Review not found: %v", err), http.StatusNotFound)
		return
	}
	log.Println("get review : ", review)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(review)
}

// POST /api/reviews/:entityType/:entityId
func addReview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userId, ok := r.Context().Value(userIDKey).(string)
	if !ok || userId == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var err error

	entityType := ps.ByName("entityType")
	entityId := ps.ByName("entityId")

	// count, err := reviewsCollection.CountDocuments(context.TODO(), bson.M{
	// 	"userId":     userId,
	// 	"entityType": entityType,
	// 	"entityId":   entityId,
	// })
	// if err != nil {
	// 	log.Printf("Error checking for existing review: %v", err)
	// 	http.Error(w, "Internal server error", http.StatusInternalServerError)
	// 	return
	// }

	// if count > 0 {
	// 	http.Error(w, "You have already reviewed this entity", http.StatusConflict)
	// 	return
	// }

	var review Review
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil || review.Rating < 1 || review.Rating > 5 || review.Comment == "" {
		http.Error(w, "Invalid review data", http.StatusBadRequest)
		return
	}

	review.ReviewID = generateID(16)
	review.UserID = userId
	review.EntityType = entityType
	review.EntityID = entityId
	review.Date = time.Now()

	inserted, err := reviewsCollection.InsertOne(context.TODO(), review)
	if err != nil {
		http.Error(w, "Failed to insert review: "+err.Error(), http.StatusInternalServerError)
		return
	}

	SetUserData("review", review.ReviewID, userId)

	mq.Emit("review-added")

	log.Println("review : ", review.ReviewID)
	log.Println("inserted review : ", inserted)

	w.WriteHeader(http.StatusCreated)
}

// PUT /api/reviews/:entityType/:entityId/:reviewId
func editReview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userId, _ := r.Context().Value(userIDKey).(string)
	reviewId := ps.ByName("reviewId")

	var review Review
	err := reviewsCollection.FindOne(context.TODO(), bson.M{"reviewid": reviewId}).Decode(&review)
	if err != nil {
		http.Error(w, fmt.Sprintf("Review not found: %v", err), http.StatusNotFound)
		return
	}

	if review.UserID != userId && !isAdmin(r.Context()) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var updatedFields map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updatedFields); err != nil {
		http.Error(w, "Invalid update data", http.StatusBadRequest)
		return
	}

	_, err = reviewsCollection.UpdateOne(
		context.TODO(),
		bson.M{"reviewid": reviewId},
		bson.M{"$set": updatedFields},
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update Review: %v", err), http.StatusInternalServerError)
		return
	}

	mq.Emit("review-edited")

	w.WriteHeader(http.StatusOK)
}

// DELETE /api/reviews/:entityType/:entityId/:reviewId
func deleteReview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userId, _ := r.Context().Value(userIDKey).(string)
	reviewId := ps.ByName("reviewId")

	var review Review
	err := reviewsCollection.FindOne(context.TODO(), bson.M{"reviewid": reviewId}).Decode(&review)
	if err != nil {
		http.Error(w, fmt.Sprintf("Review not found: %v", err), http.StatusNotFound)
		return
	}

	if review.UserID != userId && !isAdmin(r.Context()) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	_, err = reviewsCollection.DeleteOne(context.TODO(), bson.M{"reviewid": reviewId})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete review: %v", err), http.StatusInternalServerError)
		return
	}

	DelUserData("review", reviewId, userId)

	mq.Emit("review-deleted")

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
