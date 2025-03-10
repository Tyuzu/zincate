package tickets

import (
	"context"
	"encoding/json"
	"fmt"
	"naevis/db"
	"naevis/globals"
	"naevis/mq"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Create Ticket
func CreateTicket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")

	// Parse form values
	name := r.FormValue("name")
	priceStr := r.FormValue("price")
	quantityStr := r.FormValue("quantity")
	color := r.FormValue("color")

	// Validate inputs
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	if priceStr == "" {
		http.Error(w, "Price is required", http.StatusBadRequest)
		return
	}
	if quantityStr == "" {
		http.Error(w, "Quantity is required", http.StatusBadRequest)
		return
	}
	if color == "" {
		http.Error(w, "Color is required", http.StatusBadRequest)
		return
	}

	// Convert price and quantity to appropriate types
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil || price <= 0 {
		http.Error(w, "Invalid price value", http.StatusBadRequest)
		return
	}

	quantity, err := strconv.Atoi(quantityStr)
	if err != nil || quantity < 0 {
		http.Error(w, "Invalid quantity value", http.StatusBadRequest)
		return
	}

	// Create a new Ticket instance
	tick := structs.Ticket{
		EventID:  eventID,
		Name:     name,
		Price:    price,
		Quantity: quantity,
		Color:    color,
		TicketID: utils.GenerateID(12), // Ensure unique ID
	}

	// Insert ticket into MongoDB
	// collection := client.Database("eventdb").Collection("ticks")
	_, err = db.TicketsCollection.InsertOne(context.TODO(), tick)
	if err != nil {
		http.Error(w, "Failed to create ticket: "+err.Error(), http.StatusInternalServerError)
		return
	}
	m := mq.Index{EntityType: "ticket", EntityId: tick.TicketID, Action: "POST", ItemType: "event", ItemId: eventID}
	go mq.Emit("ticket-created", m)

	// Respond with the created ticket
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(tick); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetTickets(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	// cacheKey := "event:" + eventID + ":tickets"

	// // Check if the tickets are cached
	// cachedTickets, err := RdxGet(cacheKey)
	// if err == nil && cachedTickets != "" {
	// 	// If cached, return the data from Redis
	// 	w.Header().Set("Content-Type", "application/json")
	// 	w.Write([]byte(cachedTickets))
	// 	return
	// }

	// Retrieve tickets from MongoDB if not cached
	// collection := client.Database("eventdb").Collection("ticks")
	var tickList []structs.Ticket
	filter := bson.M{"eventid": eventID}
	cursor, err := db.TicketsCollection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Failed to fetch tickets", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())
	for cursor.Next(context.Background()) {
		var tick structs.Ticket
		if err := cursor.Decode(&tick); err != nil {
			http.Error(w, "Failed to decode ticket", http.StatusInternalServerError)
			return
		}
		tickList = append(tickList, tick)
	}

	if err := cursor.Err(); err != nil {
		http.Error(w, "Cursor error", http.StatusInternalServerError)
		return
	}

	// // Cache the tickets in Redis
	// ticketsJSON, _ := json.Marshal(tickList)
	// RdxSet(cacheKey, string(ticketsJSON))

	// Respond with the ticket data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tickList)
}

// Fetch a single ticketandise item
func GetTicket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	ticketID := ps.ByName("ticketid")
	// cacheKey := fmt.Sprintf("tick:%s:%s", eventID, ticketID)

	// // Check if the ticket is cached
	// cachedTicket, err := RdxGet(cacheKey)
	// if err == nil && cachedTicket != "" {
	// 	// Return cached data
	// 	w.Header().Set("Content-Type", "application/json")
	// 	w.Write([]byte(cachedTicket))
	// 	return
	// }

	// collection := client.Database("eventdb").Collection("ticks")
	var ticket structs.Ticket
	err := db.TicketsCollection.FindOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": ticketID}).Decode(&ticket)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ticket not found: %v", err), http.StatusNotFound)
		return
	}

	// // Cache the result
	// ticketJSON, _ := json.Marshal(ticket)
	// RdxSet(cacheKey, string(ticketJSON))

	// Respond with ticket data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ticket)
}

// Edit Ticket
func EditTicket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	tickID := ps.ByName("ticketid")

	// Parse incoming ticket data
	var tick structs.Ticket
	if err := json.NewDecoder(r.Body).Decode(&tick); err != nil {
		http.Error(w, "Invalid input data", http.StatusBadRequest)
		return
	}

	// Fetch the current ticket from the database
	// collection := client.Database("eventdb").Collection("ticks")
	var existingTicket structs.Ticket
	err := db.TicketsCollection.FindOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": tickID}).Decode(&existingTicket)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Ticket not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Prepare fields to update
	updateFields := bson.M{}
	if tick.Name != "" && tick.Name != existingTicket.Name {
		updateFields["name"] = tick.Name
	}
	if tick.Price > 0 && tick.Price != existingTicket.Price {
		updateFields["price"] = tick.Price
	}
	if tick.Quantity >= 0 && tick.Quantity != existingTicket.Quantity {
		updateFields["quantity"] = tick.Quantity
	}
	if tick.Color != "" && tick.Color != existingTicket.Color {
		updateFields["color"] = tick.Color
	}

	// If no fields have changed, return a response without updating
	if len(updateFields) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"message": "No changes detected for ticket",
		})
		return
	}

	// Perform the update in MongoDB
	_, err = db.TicketsCollection.UpdateOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": tickID}, bson.M{"$set": updateFields})
	if err != nil {
		http.Error(w, "Failed to update ticket: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// // Invalidate the cache for the event's tickets
	// if _, err := RdxDel("event:" + eventID + ":tickets"); err != nil {
	// 	log.Printf("Cache deletion failed for event: %s, Error: %v", eventID, err)
	// } else {
	// 	log.Printf("Cache invalidated for event: %s", eventID)
	// }
	m := mq.Index{EntityType: "ticket", EntityId: tickID, Action: "PUT", ItemType: "event", ItemId: eventID}
	go mq.Emit("ticket-edited", m)

	// Respond with success and updated fields
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Ticket updated successfully",
		"data":    updateFields,
	})
}

// Delete Ticket
func DeleteTicket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	tickID := ps.ByName("ticketid")

	// Delete the ticket from MongoDB
	// collection := client.Database("eventdb").Collection("ticks")
	_, err := db.TicketsCollection.DeleteOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": tickID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// w.WriteHeader(http.StatusNoContent)
	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Ticket deleted successfully",
	})
	// RdxDel("event:" + eventID + ":tickets") // Invalidate cache after deletion

	m := mq.Index{EntityType: "ticket", EntityId: tickID, Action: "DELETE", ItemType: "event", ItemId: eventID}
	go mq.Emit("ticket-deleted", m)
}

// Buy Ticket
func BuyTicket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	ticketID := ps.ByName("ticketid")

	// Retrieve the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}
	userID := requestingUserID

	// Decode the JSON body to get the quantity
	var requestBody struct {
		Quantity int `json:"quantity"`
	}

	// Parse the request body
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil || requestBody.Quantity <= 0 {
		http.Error(w, "Invalid quantity in the request", http.StatusBadRequest)
		return
	}
	quantityRequested := requestBody.Quantity

	// Find the ticket in the database
	// collection := client.Database("eventdb").Collection("ticks")
	var ticket structs.Ticket
	err = db.TicketsCollection.FindOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": ticketID}).Decode(&ticket)
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
	_, err = db.TicketsCollection.UpdateOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": ticketID}, update)
	if err != nil {
		http.Error(w, "Failed to update ticket quantity", http.StatusInternalServerError)
		return
	}

	m := mq.Index{}
	mq.Notify("ticket-bought", m)

	userdata.SetUserData("ticket", ticketID, userID)

	// // Broadcast WebSocket message
	// message := map[string]any{
	// 	"event":              "ticket-updated",
	// 	"eventid":            eventID,
	// 	"ticketid":           ticketID,
	// 	"quantity_remaining": ticket.Quantity - quantityRequested,
	// }
	// broadcastUpdate(message)
	// broadcastWebSocketMessage(message)

	// // After successfully decreasing the ticket quantity
	// broadcastUpdate(map[string]interface{}{
	// 	"event":              "ticket-updated",
	// 	"eventid":            eventID,
	// 	"ticketid":           ticketID,
	// 	"quantity_remaining": ticket.Quantity - quantityRequested,
	// })

	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Ticket purchased successfully",
	})
}
