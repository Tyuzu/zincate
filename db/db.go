package db

import (
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	UserCollection       *mongo.Collection
	IternaryCollection   *mongo.Collection
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
	FilesCollection *mongo.Collection
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
