package booking

import (
	"context"
	"encoding/json"
	"naevis/db"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

type Slot struct {
	Date     string `json:"date" bson:"date"` // e.g., "2025-05-01"
	Time     string `json:"time" bson:"time"` // e.g., "18:00"
	Capacity int    `json:"capacity" bson:"capacity"`
}

type Booking struct {
	Date  string `json:"date" bson:"date"`
	Time  string `json:"time" bson:"time"`
	Name  string `json:"name" bson:"name"`
	Seats int    `json:"seats" bson:"seats"`
}

func AddSlot(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var slot Slot
	if err := json.NewDecoder(r.Body).Decode(&slot); err != nil {
		http.Error(w, "Invalid input", 400)
		return
	}

	ctx := context.Background()
	coll := db.SlotCollection

	// Check for duplicate slot
	filter := bson.M{"date": slot.Date, "time": slot.Time}
	count, _ := coll.CountDocuments(ctx, filter)
	if count > 0 {
		http.Error(w, "Slot already exists", http.StatusConflict)
		return
	}

	_, err := coll.InsertOne(ctx, slot)
	if err != nil {
		http.Error(w, "DB error", 500)
		return
	}

	w.WriteHeader(201)
}

func DeleteSlot(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	date := ps.ByName("date")
	time := ps.ByName("time")

	ctx := context.Background()
	coll := db.SlotCollection

	_, err := coll.DeleteOne(ctx, bson.M{"date": date, "time": time})
	if err != nil {
		http.Error(w, "DB error", 500)
		return
	}

	w.WriteHeader(204)
}

func GetSlotsByDate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	date := ps.ByName("date")
	ctx := context.Background()

	cursor, err := db.SlotCollection.Find(ctx, bson.M{"date": date})
	if err != nil {
		http.Error(w, "DB error", 500)
		return
	}
	defer cursor.Close(ctx)

	var slots []Slot
	if err = cursor.All(ctx, &slots); err != nil {
		http.Error(w, "Parse error", 500)
		return
	}

	json.NewEncoder(w).Encode(slots)
}

func GetBookingsByDate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	date := ps.ByName("date")
	ctx := context.Background()

	cursor, err := db.BookingsCollection.Find(ctx, bson.M{"date": date})
	if err != nil {
		http.Error(w, "DB error", 500)
		return
	}
	defer cursor.Close(ctx)

	var bookings []Booking
	if err := cursor.All(ctx, &bookings); err != nil {
		http.Error(w, "Parse error", 500)
		return
	}

	json.NewEncoder(w).Encode(bookings)
}

func CreateBooking(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var booking Booking
	if err := json.NewDecoder(r.Body).Decode(&booking); err != nil {
		http.Error(w, "Invalid input", 400)
		return
	}

	if booking.Name == "" || booking.Date == "" || booking.Time == "" || booking.Seats < 1 {
		http.Error(w, "Missing or invalid fields", 400)
		return
	}

	ctx := context.Background()
	slotsColl := db.SlotCollection
	bookingsColl := db.BookingsCollection

	// 1. Check slot exists
	var slot Slot
	err := slotsColl.FindOne(ctx, bson.M{"date": booking.Date, "time": booking.Time}).Decode(&slot)
	if err != nil {
		http.Error(w, "Slot not found", 404)
		return
	}

	// 2. Sum existing bookings for this slot
	cursor, err := bookingsColl.Find(ctx, bson.M{"date": booking.Date, "time": booking.Time})
	if err != nil {
		http.Error(w, "DB error", 500)
		return
	}
	defer cursor.Close(ctx)

	totalBooked := 0
	for cursor.Next(ctx) {
		var b Booking
		cursor.Decode(&b)
		totalBooked += b.Seats
	}

	if totalBooked+booking.Seats > slot.Capacity {
		http.Error(w, "Not enough seats available", http.StatusConflict)
		return
	}

	// 3. Insert booking
	_, err = bookingsColl.InsertOne(ctx, booking)
	if err != nil {
		http.Error(w, "Could not book", 500)
		return
	}

	w.WriteHeader(201)
}
