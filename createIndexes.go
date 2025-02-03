package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func CreateIndexes(client *mongo.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Define collections
	settingsCollection := client.Database("eventdb").Collection("settings")
	reviewsCollection := client.Database("eventdb").Collection("reviews")
	// Define other collections similarly...

	// Create indexes for collections
	collectionsAndIndexes := map[*mongo.Collection][]mongo.IndexModel{
		settingsCollection: {
			{Keys: bson.D{{Key: "key", Value: 1}}}, // Example: Indexing the `key` field for settings
		},
		reviewsCollection: {
			{Keys: bson.D{{Key: "event_id", Value: 1}, {Key: "user_id", Value: 1}}}, // Compound index for event and user
		},
		// Add other collections and their indexes here
	}

	for collection, indexes := range collectionsAndIndexes {
		_, err := collection.Indexes().CreateMany(ctx, indexes)
		if err != nil {
			log.Fatalf("Failed to create indexes for collection %s: %v", collection.Name(), err)
		}
		log.Printf("Indexes created for collection %s", collection.Name())
	}
}
