package merch

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/mq"
	"naevis/stripe"
	"naevis/structs"
	"naevis/tickets"
	"naevis/userdata"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// POST /merch/event/:eventId/:merchId/payment-session
func CreateMerchPaymentSession(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	merchId := ps.ByName("merchid")
	eventId := ps.ByName("eventid")

	// Parse request body for stock
	var body struct {
		Stock int `json:"stock"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Stock < 1 {
		http.Error(w, "Invalid request or stock", http.StatusBadRequest)
		return
	}

	// Generate a Stripe payment session
	session, err := stripe.CreateMerchSession(merchId, eventId, body.Stock)
	if err != nil {
		log.Printf("Error creating payment session: %v", err)
		http.Error(w, "Failed to create payment session", http.StatusInternalServerError)
		return
	}

	// Respond with the session URL
	dataResponse := map[string]any{
		"paymentUrl": session.URL,
		"eventid":    session.EventID,
		"merchid":    session.MerchID,
		"stock":      session.Stock,
	}

	// Respond with the session URL
	response := map[string]any{
		"success": true,
		"data":    dataResponse,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// BroadcastMerchUpdate sends real-time merch updates to subscribers
func BroadcastMerchUpdate(eventId, merchId string, remainingMerchs int) {
	update := map[string]any{
		"type":            "merch_update",
		"merchId":         merchId,
		"remainingMerchs": remainingMerchs,
	}
	channel := tickets.GetUpdatesChannel(eventId)
	select {
	case channel <- update:
		// Successfully sent update
	default:
		// If the channel is full, log a warning or handle the overflow
		log.Printf("Warning: Updates channel for event %s is full. Dropping update.", eventId)
	}
}

// MerchPurchaseRequest represents the request body for purchasing merchs
type MerchPurchaseRequest struct {
	MerchID string `json:"merchId"`
	EventID string `json:"eventId"`
	Stock   int    `json:"stock"`
}

// MerchPurchaseResponse represents the response body for merch purchase confirmation
type MerchPurchaseResponse struct {
	Message string `json:"message"`
}

// ProcessMerchPayment simulates the payment processing logic
func ProcessMerchPayment(merchID, eventID string, stock int) bool {
	// Implement actual payment processing logic (e.g., calling a payment gateway)
	// For the sake of this example, we'll assume payment is always successful.
	log.Printf("Processing payment for MerchID: %s, EventID: %s, Stock: %d", merchID, eventID, stock)
	return true
}

// UpdateMerchStatus simulates updating the merch status in the database
func UpdateMerchStatus(merchID, eventID string, stock int) error {
	// Implement actual logic to update merch status in the database
	log.Printf("Updating merch status for MerchID: %s, EventID: %s, Stock: %d", merchID, eventID, stock)
	return nil
}

// ConfirmPurchase handles the POST request for confirming the merch purchase
func ConfirmMerchPurchase(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var request MerchPurchaseRequest
	// Parse the incoming JSON request
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Retrieve the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	request.EventID = ps.ByName("eventid")
	request.MerchID = ps.ByName("merchid")

	fmt.Println(request)
	// Process the payment
	paymentProcessed := ProcessMerchPayment(request.MerchID, request.EventID, request.Stock)

	if paymentProcessed {
		// Update the merch status in the database
		err = UpdateMerchStatus(request.MerchID, request.EventID, request.Stock)
		if err != nil {
			http.Error(w, "Failed to update merch status", http.StatusInternalServerError)
			return
		}

		// // Respond with a success message
		// response := MerchPurchaseResponse{
		// 	Message: "Payment successfully processed. Merch purchased.",
		// }
		// w.Header().Set("Content-Type", "application/json")
		// w.WriteHeader(http.StatusOK)
		// json.NewEncoder(w).Encode(response)
		buyxMerch(w, request, requestingUserID)
	} else {
		// If payment failed, respond with a failure message
		http.Error(w, "Payment failed", http.StatusBadRequest)
	}
}

// Buy Merch

func buyxMerch(w http.ResponseWriter, request MerchPurchaseRequest, requestingUserID string) {
	eventID := request.EventID
	merchID := request.MerchID
	stockRequested := request.Stock

	// Find the merch in the database
	// collection := client.Database("eventdb").Collection("merch")
	var merch structs.Merch // Define the Merch struct based on your schema
	err := db.MerchCollection.FindOne(context.TODO(), bson.M{"eventid": eventID, "merchid": merchID}).Decode(&merch)
	if err != nil {
		http.Error(w, "Merch not found or other error", http.StatusNotFound)
		return
	}

	// Check if there are enough merch available for purchase
	if merch.Stock < stockRequested {
		http.Error(w, "Not enough merch available for purchase", http.StatusBadRequest)
		return
	}

	// Decrease the merch stock by the requested quantity
	update := bson.M{"$inc": bson.M{"stock": -stockRequested}}
	_, err = db.MerchCollection.UpdateOne(context.TODO(), bson.M{"eventid": eventID, "merchid": merchID}, update)
	if err != nil {
		http.Error(w, "Failed to update merch stock", http.StatusInternalServerError)
		return
	}

	userdata.SetUserData("merch", merchID, requestingUserID)

	m := mq.Index{}
	mq.Notify("merch-bought", m)

	// Respond with a success message
	response := MerchPurchaseResponse{
		Message: "Payment successfully processed. Merch purchased.",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	// // Respond with success
	// w.Header().Set("Content-Type", "application/json")
	// w.WriteHeader(http.StatusOK)
	// json.NewEncoder(w).Encode(map[string]any{
	// 	"success": true,
	// 	"message": fmt.Sprintf("Successfully purchased %d of %s", stockRequested, merch.Name),
	// })
}
