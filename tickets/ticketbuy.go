package tickets

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/mq"
	"naevis/stripe"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// A global map to manage event-specific update channels
var eventUpdateChannels = struct {
	sync.RWMutex
	channels map[string]chan map[string]any
}{
	channels: make(map[string]chan map[string]any),
}

// Helper function to get or create the updates channel for an event
func GetUpdatesChannel(eventId string) chan map[string]any {
	eventUpdateChannels.RLock()
	if ch, exists := eventUpdateChannels.channels[eventId]; exists {
		eventUpdateChannels.RUnlock()
		return ch
	}
	eventUpdateChannels.RUnlock()

	// Create a new channel if not exists
	eventUpdateChannels.Lock()
	defer eventUpdateChannels.Unlock()
	newCh := make(chan map[string]any, 10) // Buffered channel
	eventUpdateChannels.channels[eventId] = newCh
	return newCh
}

// POST /ticket/event/:eventId/:ticketId/payment-session
func CreateTicketPaymentSession(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ticketId := ps.ByName("ticketid")
	eventId := ps.ByName("eventid")

	// Ensure Content-Type is application/json
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Invalid Content-Type, expected application/json", http.StatusUnsupportedMediaType)
		return
	}

	// Read request body safely
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close() // Close body after reading

	// Debugging: Print raw request body
	fmt.Println("Raw Body:", string(bodyBytes))

	// Parse request body into struct
	var body struct {
		Quantity int `json:"quantity"`
	}

	if err := json.Unmarshal(bodyBytes, &body); err != nil || body.Quantity < 1 {
		fmt.Println("Error decoding JSON:", err)
		http.Error(w, "Invalid request or quantity", http.StatusBadRequest)
		return
	}

	fmt.Printf("Decoded Body: %+v\n", body) // Debugging

	// Generate a Stripe payment session
	session, err := stripe.CreateTicketSession(ticketId, eventId, body.Quantity)
	if err != nil {
		log.Printf("Error creating payment session: %v", err)
		http.Error(w, "Failed to create payment session", http.StatusInternalServerError)
		return
	}

	// Respond with session details
	response := map[string]any{
		"success": true,
		"data": map[string]any{
			"paymentUrl": session.URL,
			"eventid":    session.EventID,
			"ticketid":   session.TicketID,
			"quantity":   session.Quantity,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GET /events/:eventId/updates
func EventUpdates(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventId := ps.ByName("eventId")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	updatesChannel := GetUpdatesChannel(eventId)
	defer func() {
		// Optionally close the channel when the connection ends
		log.Printf("Closing updates channel for event: %s", eventId)
	}()

	// Listen for updates or client disconnection
	for {
		select {
		case update := <-updatesChannel:
			jsonUpdate, _ := json.Marshal(update)
			fmt.Fprintf(w, "data: %s\n\n", jsonUpdate)
			flusher.Flush()
		case <-r.Context().Done():
			// Client disconnected
			return
		}
	}
}

// BroadcastTicketUpdate sends real-time ticket updates to subscribers
func BroadcastTicketUpdate(eventId, ticketId string, remainingTickets int) {
	update := map[string]any{
		"type":             "ticket_update",
		"ticketId":         ticketId,
		"remainingTickets": remainingTickets,
	}
	channel := GetUpdatesChannel(eventId)
	select {
	case channel <- update:
		// Successfully sent update
	default:
		// If the channel is full, log a warning or handle the overflow
		log.Printf("Warning: Updates channel for event %s is full. Dropping update.", eventId)
	}
}

// TicketPurchaseRequest represents the request body for purchasing tickets
type TicketPurchaseRequest struct {
	TicketID string `json:"ticketId"`
	EventID  string `json:"eventId"`
	Quantity int    `json:"quantity"`
}

// TicketPurchaseResponse represents the response body for ticket purchase confirmation
type TicketPurchaseResponse struct {
	Message    string `json:"message"`
	Success    string `json:"success"`
	UniqueCode string `json:"uniquecode"`
}

// ProcessTicketPayment simulates the payment processing logic
func ProcessTicketPayment(ticketID, eventID string, quantity int) bool {
	// Implement actual payment processing logic (e.g., calling a payment gateway)
	// For the sake of this example, we'll assume payment is always successful.
	log.Printf("Processing payment for TicketID: %s, EventID: %s, Quantity: %d", ticketID, eventID, quantity)
	return true
}

// UpdateTicketStatus simulates updating the ticket status in the database
func UpdateTicketStatus(ticketID, eventID string, quantity int) error {
	// Implement actual logic to update ticket status in the database
	log.Printf("Updating ticket status for TicketID: %s, EventID: %s, Quantity: %d", ticketID, eventID, quantity)
	return nil
}

// ConfirmPurchase handles the POST request for confirming the ticket purchase
func ConfirmTicketPurchase(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var request TicketPurchaseRequest

	// Parse the incoming JSON request
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Process the payment
	paymentProcessed := ProcessTicketPayment(request.TicketID, request.EventID, request.Quantity)

	if paymentProcessed {
		// Update the ticket status in the database
		err = UpdateTicketStatus(request.TicketID, request.EventID, request.Quantity)
		if err != nil {
			http.Error(w, "Failed to update ticket status", http.StatusInternalServerError)
			return
		}

		// // Respond with a success message
		// response := TicketPurchaseResponse{
		// 	Message: "Payment successfully processed. Ticket purchased.",
		// }
		// w.Header().Set("Content-Type", "application/json")
		// w.WriteHeader(http.StatusOK)
		// json.NewEncoder(w).Encode(response)
		buyxTicket(w, r, request)
	} else {
		// If payment failed, respond with a failure message
		http.Error(w, "Payment failed", http.StatusBadRequest)
	}
}

func buyxTicket(w http.ResponseWriter, r *http.Request, request TicketPurchaseRequest) {
	eventID := request.EventID
	ticketID := request.TicketID
	quantityRequested := request.Quantity

	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	var ticket structs.Ticket
	err := db.TicketsCollection.FindOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": ticketID}).Decode(&ticket)
	if err != nil {
		http.Error(w, "Ticket not found or other error", http.StatusNotFound)
		return
	}

	if ticket.Quantity < quantityRequested {
		http.Error(w, "Not enough tickets available for purchase", http.StatusBadRequest)
		return
	}

	_, err = db.TicketsCollection.UpdateOne(context.TODO(),
		bson.M{"eventid": eventID, "ticketid": ticketID},
		bson.M{"$inc": bson.M{"quantity": -quantityRequested}},
	)
	if err != nil {
		http.Error(w, "Failed to update ticket quantity", http.StatusInternalServerError)
		return
	}

	var purchasedDocs []interface{}
	var userDataDocs []structs.UserData
	var uniqueCodes []string
	now := time.Now()
	createdAt := now.Format(time.RFC3339)

	for i := 0; i < quantityRequested; i++ {
		uniqueCode := utils.GetUUID()
		uniqueCodes = append(uniqueCodes, uniqueCode)

		purchasedDocs = append(purchasedDocs, structs.PurchasedTicket{
			EventID:      eventID,
			TicketID:     ticketID,
			UserID:       requestingUserID,
			UniqueCode:   uniqueCode,
			PurchaseDate: now,
		})

		userDataDocs = append(userDataDocs, structs.UserData{
			EntityID:   uniqueCode,
			EntityType: "ticket",
			UserID:     requestingUserID,
			CreatedAt:  createdAt,
		})
	}

	_, err = db.PurchasedTicketsCollection.InsertMany(context.TODO(), purchasedDocs)
	if err != nil {
		http.Error(w, "Failed to store purchased tickets", http.StatusInternalServerError)
		return
	}

	userdata.AddUserDataBatch(userDataDocs)

	mq.Notify("ticket-bought", mq.Index{})

	response := struct {
		Message     string   `json:"message"`
		Success     string   `json:"success"`
		UniqueCodes []string `json:"uniqueCodes"`
	}{
		Message:     "Payment successfully processed. Tickets purchased.",
		Success:     "true",
		UniqueCodes: uniqueCodes,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// // Buy Ticket
// func buyxTicket(w http.ResponseWriter, r *http.Request, request TicketPurchaseRequest) {
// 	eventID := request.EventID
// 	ticketID := request.TicketID
// 	quantityRequested := request.Quantity

// 	// Retrieve the ID of the requesting user from the context
// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok {
// 		http.Error(w, "Invalid user", http.StatusBadRequest)
// 		return
// 	}

// 	// Find the ticket in the database
// 	var ticket structs.Ticket
// 	err := db.TicketsCollection.FindOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": ticketID}).Decode(&ticket)
// 	if err != nil {
// 		http.Error(w, "Ticket not found or other error", http.StatusNotFound)
// 		return
// 	}

// 	// Check if there are tickets available
// 	if ticket.Quantity <= 0 {
// 		http.Error(w, "No tickets available for purchase", http.StatusBadRequest)
// 		return
// 	}

// 	// Check if the requested quantity is available
// 	if ticket.Quantity < quantityRequested {
// 		http.Error(w, "Not enough tickets available for purchase", http.StatusBadRequest)
// 		return
// 	}

// 	// Decrease the ticket quantity
// 	update := bson.M{"$inc": bson.M{"quantity": -quantityRequested}}
// 	_, err = db.TicketsCollection.UpdateOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": ticketID}, update)
// 	if err != nil {
// 		http.Error(w, "Failed to update ticket quantity", http.StatusInternalServerError)
// 		return
// 	}

// 	// Generate a unique code for the ticket
// 	uniqueCode := utils.GetUUID() // Generate unique ID

// 	// Store the unique code in the database (optional: create a separate collection for purchased tickets)
// 	purchasedTicket := structs.PurchasedTicket{
// 		EventID:      eventID,
// 		TicketID:     ticketID,
// 		UserID:       requestingUserID,
// 		UniqueCode:   uniqueCode,
// 		PurchaseDate: time.Now(),
// 	}
// 	_, err = db.PurchasedTicketsCollection.InsertOne(context.TODO(), purchasedTicket)
// 	if err != nil {
// 		http.Error(w, "Failed to store purchased ticket", http.StatusInternalServerError)
// 		return
// 	}

// 	// Notify other systems about the ticket purchase
// 	m := mq.Index{}
// 	mq.Notify("ticket-bought", m)

// 	userdata.SetUserData("ticket", uniqueCode, requestingUserID)

// 	// Respond with a success message and the unique code
// 	response := TicketPurchaseResponse{
// 		Message:    "Payment successfully processed. Ticket purchased.",
// 		Success:    "true",
// 		UniqueCode: uniqueCode,
// 	}
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusOK)
// 	json.NewEncoder(w).Encode(response)
// }

// // Buy Ticket
// func buyxTicket(w http.ResponseWriter, r *http.Request, request TicketPurchaseRequest) {
// 	eventID := request.EventID
// 	ticketID := request.TicketID
// 	quantityRequested := request.Quantity

// 	// Retrieve the ID of the requesting user from the context
// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok {
// 		http.Error(w, "Invalid user", http.StatusBadRequest)
// 		return
// 	}

// 	// Find the ticket in the database
// 	// collection := client.Database("eventdb").Collection("ticks")
// 	var ticket structs.Ticket
// 	err := db.TicketsCollection.FindOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": ticketID}).Decode(&ticket)
// 	if err != nil {
// 		http.Error(w, "Ticket not found or other error", http.StatusNotFound)
// 		return
// 	}

// 	// Check if there are tickets available
// 	if ticket.Quantity <= 0 {
// 		http.Error(w, "No tickets available for purchase", http.StatusBadRequest)
// 		return
// 	}

// 	// Check if the requested quantity is available
// 	if ticket.Quantity < quantityRequested {
// 		http.Error(w, "Not enough tickets available for purchase", http.StatusBadRequest)
// 		return
// 	}

// 	// Decrease the ticket quantity
// 	update := bson.M{"$inc": bson.M{"quantity": -quantityRequested}}
// 	_, err = db.TicketsCollection.UpdateOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": ticketID}, update)
// 	if err != nil {
// 		http.Error(w, "Failed to update ticket quantity", http.StatusInternalServerError)
// 		return
// 	}

// 	// // Respond with success
// 	// w.Header().Set("Content-Type", "application/json")
// 	// w.WriteHeader(http.StatusOK)
// 	// json.NewEncoder(w).Encode(map[string]interface{}{
// 	// 	"success": true,
// 	// 	"message": "Ticket purchased successfully",
// 	// })

// 	m := mq.Index{}
// 	mq.Notify("ticket-bought", m)

// 	userdata.SetUserData("ticket", ticketID, requestingUserID)

// 	// Respond with a success message
// 	response := TicketPurchaseResponse{
// 		Message: "Payment successfully processed. Ticket purchased.",
// 		Success: "true",
// 	}
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusOK)
// 	json.NewEncoder(w).Encode(response)
// }

// Get Available Seats
func GetAvailableSeats(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")

	filter := bson.M{"event_id": eventID}
	var ticket struct {
		Seats []struct {
			SeatID string `bson:"seat_id"`
			Status string `bson:"status"`
		} `bson:"seats"`
	}

	err := db.TicketsCollection.FindOne(context.Background(), filter).Decode(&ticket)
	if err != nil {
		http.Error(w, `{"error": "No tickets found for this event"}`, http.StatusNotFound)
		return
	}

	var availableSeats []string
	for _, seat := range ticket.Seats {
		if seat.Status == "available" {
			availableSeats = append(availableSeats, seat.SeatID)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"seats": availableSeats})
}

// Lock Seats
func LockSeats(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	var request struct {
		UserID string   `json:"user_id"`
		Seats  []string `json:"seats"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	filter := bson.M{"event_id": eventID, "seats.seat_id": bson.M{"$in": request.Seats}}
	update := bson.M{"$set": bson.M{"seats.$[].status": "locked", "seats.$[].user_id": request.UserID}}

	_, err := db.TicketsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		http.Error(w, `{"error": "Failed to lock seats"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "Seats locked successfully"})
}

// Unlock Seats
func UnlockSeats(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	var request struct {
		UserID string   `json:"user_id"`
		Seats  []string `json:"seats"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	filter := bson.M{"event_id": eventID, "seats.seat_id": bson.M{"$in": request.Seats}, "seats.user_id": request.UserID}
	update := bson.M{"$set": bson.M{"seats.$[].status": "available", "seats.$[].user_id": nil}}

	_, err := db.TicketsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		http.Error(w, `{"error": "Failed to unlock seats"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "Seats unlocked successfully"})
}

// Confirm Seat Purchase
func ConfirmSeatPurchase(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	ticketID := ps.ByName("ticketid")

	var request struct {
		UserID string   `json:"user_id"`
		Seats  []string `json:"seats"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	filter := bson.M{"_id": ticketID, "event_id": eventID, "seats.seat_id": bson.M{"$in": request.Seats}}
	var ticket struct {
		Seats []struct {
			SeatID string `bson:"seat_id"`
			Status string `bson:"status"`
			UserID string `bson:"user_id"`
		} `bson:"seats"`
	}

	err := db.TicketsCollection.FindOne(context.Background(), filter).Decode(&ticket)
	if err != nil {
		http.Error(w, `{"error": "Ticket or seats not found"}`, http.StatusNotFound)
		return
	}

	for _, seat := range ticket.Seats {
		if seat.Status != "locked" || seat.UserID != request.UserID {
			http.Error(w, `{"error": "Some seats are not properly locked or have been taken"}`, http.StatusConflict)
			return
		}
	}

	update := bson.M{"$set": bson.M{"seats.$[].status": "booked"}}

	_, err = db.TicketsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		http.Error(w, `{"error": "Failed to confirm purchase"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "Ticket purchased successfully"})
}
