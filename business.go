// main.go
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func AddBusinessHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var business Business
	if err := json.NewDecoder(r.Body).Decode(&business); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := businessesCollection.InsertOne(ctx, business)
	if err != nil {
		http.Error(w, "Failed to add business", http.StatusInternalServerError)
		return
	}

	business.ID = result.InsertedID.(primitive.ObjectID)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(business)
}

func GetBusinessHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid business ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var business Business
	err = businessesCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&business)
	if err != nil {
		http.Error(w, "Business not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(business)
}

func GetBusinesses(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

	// Create the sort order (descending by createdAt)
	sortOrder := bson.D{{Key: "created_at", Value: -1}}

	// Find businesses with pagination and sorting
	cursor, err := businessesCollection.Find(context.TODO(), bson.M{}, &options.FindOptions{
		Skip:  &int64Skip,
		Limit: &int64Limit,
		Sort:  sortOrder,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var businesses []Business
	if err = cursor.All(context.TODO(), &businesses); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if businesses == nil {
		businesses = []Business{}
	}

	// Encode the list of businesses as JSON and write to the response
	if err := json.NewEncoder(w).Encode(businesses); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func BookSlotHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	businessID, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid business ID", http.StatusBadRequest)
		return
	}

	var booking Booking
	if err := json.NewDecoder(r.Body).Decode(&booking); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	booking.BusinessID = businessID

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := bookingsCollection.InsertOne(ctx, booking)
	if err != nil {
		http.Error(w, "Failed to book slot", http.StatusInternalServerError)
		return
	}

	booking.ID = result.InsertedID.(primitive.ObjectID)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(booking)
}

func GetMenuHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	businessID, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid business ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var menu []bson.M
	cursor, err := menusCollection.Find(ctx, bson.M{"business_id": businessID})
	if err != nil {
		http.Error(w, "Failed to fetch menu", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &menu); err != nil {
		http.Error(w, "Failed to parse menu", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(menu)
}

func GetPromotionsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	businessID, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid business ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var promotions []bson.M
	cursor, err := promotionsCollection.Find(ctx, bson.M{"business_id": businessID})
	if err != nil {
		http.Error(w, "Failed to fetch promotions", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &promotions); err != nil {
		http.Error(w, "Failed to parse promotions", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(promotions)
}

func RegisterOwnerHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var owner Owner
	if err := json.NewDecoder(r.Body).Decode(&owner); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := ownersCollection.InsertOne(ctx, owner)
	if err != nil {
		http.Error(w, "Failed to register owner", http.StatusInternalServerError)
		return
	}

	owner.ID = result.InsertedID.(primitive.ObjectID)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(owner)
}

func LoginOwnerHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var credentials struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var owner Owner
	err := ownersCollection.FindOne(ctx, bson.M{"email": credentials.Email, "password": credentials.Password}).Decode(&owner)
	if err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(owner)
}

func AddBusinessByOwnerHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var business Business
	if err := json.NewDecoder(r.Body).Decode(&business); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := businessesCollection.InsertOne(ctx, business)
	if err != nil {
		http.Error(w, "Failed to add business", http.StatusInternalServerError)
		return
	}

	business.ID = result.InsertedID.(primitive.ObjectID)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(business)
}

func UpdateBusinessHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid business ID", http.StatusBadRequest)
		return
	}

	var update bson.M
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := businessesCollection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	if err != nil {
		http.Error(w, "Failed to update business", http.StatusInternalServerError)
		return
	}

	if result.MatchedCount == 0 {
		http.Error(w, "Business not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Business updated successfully"))
}

func DeleteBusinessHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid business ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := businessesCollection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		http.Error(w, "Failed to delete business", http.StatusInternalServerError)
		return
	}

	if result.DeletedCount == 0 {
		http.Error(w, "Business not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Business deleted successfully"))
}

func AddOrUpdateMenuHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid business ID", http.StatusBadRequest)
		return
	}

	var menuItem MenuItem
	if err := json.NewDecoder(r.Body).Decode(&menuItem); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	menuItem.ID = primitive.NewObjectID()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = menusCollection.UpdateOne(ctx, bson.M{"business_id": id}, bson.M{"$push": bson.M{"items": menuItem}}, options.Update().SetUpsert(true))
	if err != nil {
		http.Error(w, "Failed to add/update menu item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(menuItem)
}

func DeleteMenuItemHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	businessID, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid business ID", http.StatusBadRequest)
		return
	}

	itemID, err := primitive.ObjectIDFromHex(ps.ByName("itemId"))
	if err != nil {
		http.Error(w, "Invalid menu item ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = menusCollection.UpdateOne(ctx, bson.M{"business_id": businessID}, bson.M{"$pull": bson.M{"items": bson.M{"_id": itemID}}})
	if err != nil {
		http.Error(w, "Failed to delete menu item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Menu item deleted successfully"))
}

func AddPromotionHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid business ID", http.StatusBadRequest)
		return
	}

	var promotion Promotion
	if err := json.NewDecoder(r.Body).Decode(&promotion); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	promotion.ID = primitive.NewObjectID()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = promotionsCollection.UpdateOne(ctx, bson.M{"business_id": id}, bson.M{"$push": bson.M{"promotions": promotion}}, options.Update().SetUpsert(true))
	if err != nil {
		http.Error(w, "Failed to add promotion", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(promotion)
}

func DeletePromotionHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	businessID, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid business ID", http.StatusBadRequest)
		return
	}

	promoID, err := primitive.ObjectIDFromHex(ps.ByName("promoId"))
	if err != nil {
		http.Error(w, "Invalid promotion ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = promotionsCollection.UpdateOne(ctx, bson.M{"business_id": businessID}, bson.M{"$pull": bson.M{"promotions": bson.M{"_id": promoID}}})
	if err != nil {
		http.Error(w, "Failed to delete promotion", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Promotion deleted successfully"))
}

func ViewBookingsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid business ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := bookingsCollection.Find(ctx, bson.M{"business_id": id})
	if err != nil {
		http.Error(w, "Failed to retrieve bookings", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var bookings []Booking
	if err = cursor.All(ctx, &bookings); err != nil {
		http.Error(w, "Failed to parse bookings", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(bookings)
}

func CancelBookingHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	businessID, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid business ID", http.StatusBadRequest)
		return
	}

	bookingID, err := primitive.ObjectIDFromHex(ps.ByName("bookingId"))
	if err != nil {
		http.Error(w, "Invalid booking ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = bookingsCollection.DeleteOne(ctx, bson.M{"_id": bookingID, "business_id": businessID})
	if err != nil {
		http.Error(w, "Failed to cancel booking", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Booking canceled successfully"))
}
