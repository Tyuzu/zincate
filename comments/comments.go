package comments

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"naevis/models"
	"naevis/utils"
)

var commentsColl *mongo.Collection

func InitCommentHandler(db *mongo.Database) {
	commentsColl = db.Collection("comments")
}

func CreateComment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	comment := models.Comment{
		EntityType: entityType,
		EntityID:   entityID,
		Content:    body.Content,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	res, err := commentsColl.InsertOne(context.TODO(), comment)
	if err != nil {
		http.Error(w, "DB insert failed", http.StatusInternalServerError)
		return
	}
	comment.ID = res.InsertedID.(primitive.ObjectID).Hex()

	utils.RespondWithJSON(w, http.StatusOK, comment)
}

func GetComments(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")

	filter := bson.M{"entity_type": entityType, "entity_id": entityID}
	cursor, err := commentsColl.Find(context.TODO(), filter)
	if err != nil {
		http.Error(w, "DB find failed", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var comments []models.Comment
	if err := cursor.All(context.TODO(), &comments); err != nil {
		http.Error(w, "Cursor decode failed", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, comments)
}

func UpdateComment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	commentID := ps.ByName("commentid")

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	objID, err := primitive.ObjectIDFromHex(commentID)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	update := bson.M{
		"$set": bson.M{
			"content":    body.Content,
			"updated_at": time.Now(),
		},
	}

	_, err = commentsColl.UpdateByID(context.TODO(), objID, update)
	if err != nil {
		http.Error(w, "DB update failed", http.StatusInternalServerError)
		return
	}

	var updated models.Comment
	if err := commentsColl.FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&updated); err != nil {
		http.Error(w, "Fetch failed", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, updated)
}

func DeleteComment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	commentID := ps.ByName("commentid")

	objID, err := primitive.ObjectIDFromHex(commentID)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	_, err = commentsColl.DeleteOne(context.TODO(), bson.M{"_id": objID})
	if err != nil {
		http.Error(w, "Delete failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
