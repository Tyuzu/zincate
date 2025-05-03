package tickets

import (
	"context"
	"encoding/json"
	"naevis/db"
	"net/http"
	_ "net/http/pprof"

	"go.mongodb.org/mongo-driver/bson"
)

// import (
// 	"sync"

// 	"github.com/gorilla/websocket"
// )

// // var upgrader = websocket.Upgrader{
// // 	CheckOrigin: func(r *http.Request) bool {
// // 		return true // Adjust for production security
// // 	},
// // }

// var connections = struct {
// 	sync.Mutex
// 	clients map[*websocket.Conn]bool
// }{clients: make(map[*websocket.Conn]bool)}

// // func wsHandler(w http.ResponseWriter, r *http.Request) {
// // 	conn, err := upgrader.Upgrade(w, r, nil)
// // 	if err != nil {
// // 		http.Error(w, "Could not open WebSocket connection", http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Add the connection to the list of active clients
// // 	connections.Lock()
// // 	connections.clients[conn] = true
// // 	connections.Unlock()

// // 	// Listen for messages from the client
// // 	go func(c *websocket.Conn) {
// // 		defer func() {
// // 			connections.Lock()
// // 			delete(connections.clients, c)
// // 			connections.Unlock()
// // 			c.Close()
// // 		}()

// // 		for {
// // 			_, _, err := c.ReadMessage()
// // 			if err != nil {
// // 				break
// // 			}
// // 		}
// // 	}(conn)
// // }

// func broadcastUpdate(message any) {
// 	connections.Lock()
// 	defer connections.Unlock()

// 	for conn := range connections.clients {
// 		err := conn.WriteJSON(message)
// 		if err != nil {
// 			conn.Close()
// 			delete(connections.clients, conn)
// 		}
// 	}
// }

// // OR
// // // Publish update to Redis
// // redisClient.Publish("updates", jsonString)

// // go func() {
// // 	pubsub := redisClient.Subscribe("updates")
// // 	defer pubsub.Close()

// // 	for {
// // 		msg, err := pubsub.ReceiveMessage()
// // 		if err != nil {
// // 			continue
// // 		}

// // 		var update map[string]any
// // 		json.Unmarshal([]byte(msg.Payload), &update)
// // 		broadcastUpdate(update)
// // 	}
// // }()

func ResellTicketHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TicketID string `json:"ticketId"`
		Price    string `json:"price"`
		UserID   string `json:"userId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Update the ticket in the database
	_, err := db.TicketsCollection.UpdateOne(context.TODO(),
		bson.M{"ticket_id": req.TicketID, "owner_id": req.UserID},
		bson.M{"$set": bson.M{"is_resold": true, "resale_price": req.Price}},
	)

	if err != nil {
		http.Error(w, "Failed to list ticket for resale", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
