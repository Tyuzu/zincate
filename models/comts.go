package models

import "time"

type Comment struct {
	ID         string    `json:"_id" bson:"_id,omitempty"`
	EntityType string    `json:"entityType" bson:"entity_type"`
	EntityID   string    `json:"entityId" bson:"entity_id"`
	Content    string    `json:"content" bson:"content"`
	CreatedAt  time.Time `json:"createdAt" bson:"created_at"`
	UpdatedAt  time.Time `json:"updatedAt" bson:"updated_at"`
}
