package userdata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/db"
	"naevis/structs"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

var ValidEntityTypes = map[string]bool{
	"userhome":   true,
	"place":      true,
	"event":      true,
	"feedpost":   true,
	"media":      true,
	"ticket":     true,
	"merch":      true,
	"review":     true,
	"comment":    true,
	"like":       true,
	"favourite":  true,
	"booking":    true,
	"blogpost":   true,
	"collection": true,
}

func IsValidEntityType(entityType string) bool {
	return ValidEntityTypes[entityType]
}
func SetUserData(dataType string, dataId string, userId string) {
	fmt.Println("set dataType : ", dataType)
	fmt.Println("set dataId : ", dataId)
	fmt.Println("set userId : ", userId)
	AddUserData(dataType, dataId, userId)
}

func DelUserData(dataType string, dataId string, userId string) {
	fmt.Println("del dataType : ", dataType)
	fmt.Println("del dataId : ", dataId)
	fmt.Println("del userId : ", userId)
	RemUserData(dataType, dataId, userId)
}

func AddUserData(entityType, entityId, userId string) {
	var content structs.UserData
	content.EntityID = entityId
	content.EntityType = entityType
	content.UserID = userId
	content.CreatedAt = time.Now().Format(time.RFC3339)
	// Insert the content into MongoDB
	_, err := db.UserDataCollection.InsertOne(context.TODO(), content)
	if err != nil {
		log.Printf("Error inserting content: %v", err)
		return
	}
}

func RemUserData(entityType, entityId, userId string) {
	// Delete the content from MongoDB
	_, err := db.UserDataCollection.DeleteOne(context.TODO(), bson.M{"entity_id": entityId, "entity_type": entityType, "userid": userId})
	if err != nil {
		log.Printf("Error deleting content: %v", err)
		return
	}
}

func GetUserProfileData(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username := ps.ByName("username")

	// Parse the query parameter for the entity type
	entityType := r.URL.Query().Get("entity_type")
	if entityType == "" {
		http.Error(w, "Entity type is required", http.StatusBadRequest)
		return
	}

	// Validate the entity type
	if !IsValidEntityType(entityType) {
		http.Error(w, "Invalid entity type", http.StatusBadRequest)
		return
	}

	// Fetch user data from MongoDB
	filter := bson.M{"entity_type": entityType, "userid": username}
	cursor, err := db.UserDataCollection.Find(context.TODO(), filter)
	if err != nil {
		http.Error(w, "Failed to fetch user data", http.StatusInternalServerError)
		log.Printf("Error fetching user data: %v", err)
		return
	}
	defer cursor.Close(context.TODO())

	var results []structs.UserData
	if err := cursor.All(context.TODO(), &results); err != nil {
		http.Error(w, "Failed to decode user data", http.StatusInternalServerError)
		log.Printf("Error decoding user data: %v", err)
		return
	}
	fmt.Println(results)
	if len(results) == 0 {
		results = []structs.UserData{}
	}

	// Respond with the results
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Printf("Error encoding response: %v", err)
		return
	}
}

func AddUserDataBatch(docs []structs.UserData) {
	var toInsert []interface{}
	for _, doc := range docs {
		toInsert = append(toInsert, doc)
	}
	if len(toInsert) == 0 {
		return
	}
	_, err := db.UserDataCollection.InsertMany(context.TODO(), toInsert)
	if err != nil {
		log.Printf("Error inserting batch user data: %v", err)
	}
}
