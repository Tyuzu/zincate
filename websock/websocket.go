package websock

import (
	"log"
	"naevis/structs"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

// WebSocket handler (remains unchanged).
func WsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	conn, err := structs.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket Upgrade:", err)
		return
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}
		log.Printf("Received: %s", msg)

		// In production, send real-time updates.
		if err = conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Println("WebSocket write error:", err)
			break
		}
	}
}

func WsBroadcast(chatID string, message interface{}) {
	// msgData, _ := json.Marshal(message)
	// log.Println(chatID, message, msgData)
	// for _, conn := range activeConnections[chatID] {
	// 	conn.WriteMessage(websocket.TextMessage, msgData)
	// }
}

// func wsBroadcast(chatID int, message interface{}) {
// 	msgData, _ := json.Marshal(message)
// 	// Loop through all active connections for this chat and write the message.
// 	// activeConnections is assumed to be a map[int][]*websocket.Conn or similar.
// 	for _, conn := range activeConnections[chatID] {
// 		conn.WriteMessage(websocket.TextMessage, msgData)
// 	}
// }
