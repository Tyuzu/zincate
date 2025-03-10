package menu

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

// POST /menu/event/:placeId/:menuId/payment-session
func CreateMenuPaymentSession(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	menuId := ps.ByName("menuid")
	placeId := ps.ByName("placeid")

	// Parse request body for stock
	var body struct {
		Stock int `json:"stock"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Stock < 1 {
		http.Error(w, "Invalid request or stock", http.StatusBadRequest)
		return
	}

	// Generate a Stripe payment session
	session, err := stripe.CreateMenuSession(menuId, placeId, body.Stock)
	if err != nil {
		log.Printf("Error creating payment session: %v", err)
		http.Error(w, "Failed to create payment session", http.StatusInternalServerError)
		return
	}

	// Respond with the session URL
	dataResponse := map[string]any{
		"paymentUrl": session.URL,
		"placeid":    session.PlaceID,
		"menuid":     session.MenuID,
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

// BroadcastMenuUpdate sends real-time menu updates to subscribers
func BroadcastMenuUpdate(placeId, menuId string, remainingMenus int) {
	update := map[string]any{
		"type":           "menu_update",
		"menuId":         menuId,
		"remainingMenus": remainingMenus,
	}
	channel := tickets.GetUpdatesChannel(placeId)
	select {
	case channel <- update:
		// Successfully sent update
	default:
		// If the channel is full, log a warning or handle the overflow
		log.Printf("Warning: Updates channel for event %s is full. Dropping update.", placeId)
	}
}

// MenuPurchaseRequest represents the request body for purchasing menus
type MenuPurchaseRequest struct {
	MenuID  string `json:"menuId"`
	PlaceId string `json:"placeId"`
	Stock   int    `json:"stock"`
}

// MenuPurchaseResponse represents the response body for menu purchase confirmation
type MenuPurchaseResponse struct {
	Message string `json:"message"`
}

// ProcessMenuPayment simulates the payment processing logic
func ProcessMenuPayment(menuID, placeId string, stock int) bool {
	// Implement actual payment processing logic (e.g., calling a payment gateway)
	// For the sake of this example, we'll assume payment is always successful.
	log.Printf("Processing payment for MenuID: %s, PlaceId: %s, Stock: %d", menuID, placeId, stock)
	return true
}

// UpdateMenuStatus simulates updating the menu status in the database
func UpdateMenuStatus(menuID, placeId string, stock int) error {
	// Implement actual logic to update menu status in the database
	log.Printf("Updating menu status for MenuID: %s, PlaceId: %s, Stock: %d", menuID, placeId, stock)
	return nil
}

// ConfirmPurchase handles the POST request for confirming the menu purchase
func ConfirmMenuPurchase(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var request MenuPurchaseRequest
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

	request.PlaceId = ps.ByName("placeid")
	request.MenuID = ps.ByName("menuid")

	fmt.Println(request)
	// Process the payment
	paymentProcessed := ProcessMenuPayment(request.MenuID, request.PlaceId, request.Stock)

	if paymentProcessed {
		// Update the menu status in the database
		err = UpdateMenuStatus(request.MenuID, request.PlaceId, request.Stock)
		if err != nil {
			http.Error(w, "Failed to update menu status", http.StatusInternalServerError)
			return
		}

		// // Respond with a success message
		// response := MenuPurchaseResponse{
		// 	Message: "Payment successfully processed. Menu purchased.",
		// }
		// w.Header().Set("Content-Type", "application/json")
		// w.WriteHeader(http.StatusOK)
		// json.NewEncoder(w).Encode(response)
		buyxMenu(w, request, requestingUserID)
	} else {
		// If payment failed, respond with a failure message
		http.Error(w, "Payment failed", http.StatusBadRequest)
	}
}

// Buy Menu

func buyxMenu(w http.ResponseWriter, request MenuPurchaseRequest, requestingUserID string) {
	placeId := request.PlaceId
	menuID := request.MenuID
	stockRequested := request.Stock

	// Find the menu in the database
	// collection := client.Database("eventdb").Collection("menu")
	var menu structs.Menu // Define the Menu struct based on your schema
	err := db.MenuCollection.FindOne(context.TODO(), bson.M{"placeid": placeId, "menuid": menuID}).Decode(&menu)
	if err != nil {
		http.Error(w, "Menu not found or other error", http.StatusNotFound)
		return
	}

	// Check if there are enough menu available for purchase
	if menu.Stock < stockRequested {
		http.Error(w, "Not enough menu available for purchase", http.StatusBadRequest)
		return
	}

	// Decrease the menu stock by the requested quantity
	update := bson.M{"$inc": bson.M{"stock": -stockRequested}}
	_, err = db.MenuCollection.UpdateOne(context.TODO(), bson.M{"placeid": placeId, "menuid": menuID}, update)
	if err != nil {
		http.Error(w, "Failed to update menu stock", http.StatusInternalServerError)
		return
	}

	userdata.SetUserData("menu", menuID, requestingUserID)

	m := mq.Index{}
	mq.Notify("menu-bought", m)

	// Respond with a success message
	response := MenuPurchaseResponse{
		Message: "Payment successfully processed. Menu purchased.",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	// // Respond with success
	// w.Header().Set("Content-Type", "application/json")
	// w.WriteHeader(http.StatusOK)
	// json.NewEncoder(w).Encode(map[string]any{
	// 	"success": true,
	// 	"message": fmt.Sprintf("Successfully purchased %d of %s", stockRequested, menu.Name),
	// })
}
