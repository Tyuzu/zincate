package tickets

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/structs"
	"net/http"
	_ "net/http/pprof"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func GetTicketSeats(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	ticketID := ps.ByName("ticketid")

	var ticket structs.Ticket
	err := db.TicketsCollection.FindOne(context.TODO(), bson.M{
		"eventid":  eventID,
		"ticketid": ticketID,
	}).Decode(&ticket)

	if err != nil {
		http.Error(w, "Ticket not found", http.StatusNotFound)
		return
	}

	// Return seats list (assuming it's stored in a `Seats []string` field)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"seats":   ticket.Seats,
	})
}
