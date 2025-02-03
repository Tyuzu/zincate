package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/mq"
	"naevis/stripe"
	"net/http"
	"sync"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// A global map to manage event-specific update channels
var eventUpdateChannels = struct {
	sync.RWMutex
	channels map[string]chan map[string]interface{}
}{
	channels: make(map[string]chan map[string]interface{}),
}

// Helper function to get or create the updates channel for an event
func GetUpdatesChannel(eventId string) chan map[string]interface{} {
	eventUpdateChannels.RLock()
	if ch, exists := eventUpdateChannels.channels[eventId]; exists {
		eventUpdateChannels.RUnlock()
		return ch
	}
	eventUpdateChannels.RUnlock()

	// Create a new channel if not exists
	eventUpdateChannels.Lock()
	defer eventUpdateChannels.Unlock()
	newCh := make(chan map[string]interface{}, 10) // Buffered channel
	eventUpdateChannels.channels[eventId] = newCh
	return newCh
}

// POST /ticket/event/:eventId/:ticketId/payment-session
func CreateTicketPaymentSession(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ticketId := ps.ByName("ticketid")
	eventId := ps.ByName("eventid")

	// Parse request body for quantity
	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Quantity < 1 {
		http.Error(w, "Invalid request or quantity", http.StatusBadRequest)
		return
	}

	// Generate a Stripe payment session
	session, err := stripe.CreateTicketSession(ticketId, eventId, body.Quantity)
	if err != nil {
		log.Printf("Error creating payment session: %v", err)
		http.Error(w, "Failed to create payment session", http.StatusInternalServerError)
		return
	}

	// Respond with the session URL
	dataResponse := map[string]interface{}{
		"paymentUrl": session.URL,
		"eventid":    session.EventID,
		"ticketid":   session.TicketID,
		"quantity":   session.Quantity,
	}

	// Respond with the session URL
	response := map[string]interface{}{
		"success": true,
		"data":    dataResponse,
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
	update := map[string]interface{}{
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
	Message string `json:"message"`
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

// Buy Ticket
func buyxTicket(w http.ResponseWriter, r *http.Request, request TicketPurchaseRequest) {
	eventID := request.EventID
	ticketID := request.TicketID
	quantityRequested := request.Quantity

	// Retrieve the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(userIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	// Find the ticket in the database
	// collection := client.Database("eventdb").Collection("ticks")
	var ticket Ticket
	err := ticketsCollection.FindOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": ticketID}).Decode(&ticket)
	if err != nil {
		http.Error(w, "Ticket not found or other error", http.StatusNotFound)
		return
	}

	// Check if there are tickets available
	if ticket.Quantity <= 0 {
		http.Error(w, "No tickets available for purchase", http.StatusBadRequest)
		return
	}

	// Check if the requested quantity is available
	if ticket.Quantity < quantityRequested {
		http.Error(w, "Not enough tickets available for purchase", http.StatusBadRequest)
		return
	}

	// Decrease the ticket quantity
	update := bson.M{"$inc": bson.M{"quantity": -quantityRequested}}
	_, err = ticketsCollection.UpdateOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": ticketID}, update)
	if err != nil {
		http.Error(w, "Failed to update ticket quantity", http.StatusInternalServerError)
		return
	}

	// // Respond with success
	// w.Header().Set("Content-Type", "application/json")
	// w.WriteHeader(http.StatusOK)
	// json.NewEncoder(w).Encode(map[string]interface{}{
	// 	"success": true,
	// 	"message": "Ticket purchased successfully",
	// })

	mq.Emit("ticket-bought")

	SetUserData("ticket", ticketID, requestingUserID)

	// Respond with a success message
	response := TicketPurchaseResponse{
		Message: "Payment successfully processed. Ticket purchased.",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
