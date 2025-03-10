package db

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	UserCollection *mongo.Collection
	// ProfilesCollection *mongo.Collection
	UserDataCollection   *mongo.Collection
	TicketsCollection    *mongo.Collection
	ReviewsCollection    *mongo.Collection
	SettingsCollection   *mongo.Collection
	FollowingsCollection *mongo.Collection
	PlacesCollection     *mongo.Collection
	// BusinessesCollection *mongo.Collection
	// BookingsCollection   *mongo.Collection
	// MenusCollection      *mongo.Collection
	// PromotionsCollection *mongo.Collection
	// OwnersCollection     *mongo.Collection
	PostsCollection *mongo.Collection
	// SeatsCollection      *mongo.Collection
	MerchCollection      *mongo.Collection
	MenuCollection       *mongo.Collection
	ActivitiesCollection *mongo.Collection
	EventsCollection     *mongo.Collection
	// GigsCollection       *mongo.Collection
	MediaCollection *mongo.Collection
	// BlogCollection       *mongo.Collection
	Client *mongo.Client
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	// Get the MongoDB URI from the environment variable
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		log.Fatalf("MONGODB_URI environment variable is not set")
	}

	// Use the SetServerAPIOptions() method to set the version of the Stable API on the client
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(mongoURI).SetServerAPIOptions(serverAPI)

	// Create a new client and connect to the server
	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		panic(err)
	}

	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	// Send a ping to confirm a successful connection
	if err := client.Database("admin").RunCommand(context.TODO(), bson.D{{Key: "ping", Value: 1}}).Err(); err != nil {
		panic(err)
	}
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")

	ClientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	Client, err = mongo.Connect(context.TODO(), ClientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// CreateIndexes(Client)
	SettingsCollection = Client.Database("eventdb").Collection("settings")
	ReviewsCollection = Client.Database("eventdb").Collection("reviews")
	FollowingsCollection = Client.Database("eventdb").Collection("followings")
	// ProfilesCollection = Client.Database("eventdb").Collection("users")
	UserCollection = Client.Database("eventdb").Collection("users")
	UserDataCollection = Client.Database("eventdb").Collection("userdata")
	TicketsCollection = Client.Database("eventdb").Collection("ticks")
	PlacesCollection = Client.Database("eventdb").Collection("places")
	// BusinessesCollection = Client.Database("eventdb").Collection("businesses")
	// BookingsCollection = Client.Database("eventdb").Collection("bookings")
	// MenusCollection = Client.Database("eventdb").Collection("menus")
	// PromotionsCollection = Client.Database("eventdb").Collection("promotions")
	// OwnersCollection = Client.Database("eventdb").Collection("owners")
	PostsCollection = Client.Database("eventdb").Collection("posts")
	// SeatsCollection = Client.Database("eventdb").Collection("seats")
	MerchCollection = Client.Database("eventdb").Collection("merch")
	MenuCollection = Client.Database("eventdb").Collection("menu")
	ActivitiesCollection = Client.Database("eventdb").Collection("activities")
	EventsCollection = Client.Database("eventdb").Collection("events")
	// GigsCollection = Client.Database("eventdb").Collection("gigs")
	MediaCollection = Client.Database("eventdb").Collection("media")
	// BlogCollection = Client.Database("eventdb").Collection("blogs")
}
