package db

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

var (
	MapsCollection             *mongo.Collection
	UserCollection             *mongo.Collection
	ItineraryCollection        *mongo.Collection
	UserDataCollection         *mongo.Collection
	TicketsCollection          *mongo.Collection
	PurchasedTicketsCollection *mongo.Collection
	ReviewsCollection          *mongo.Collection
	SettingsCollection         *mongo.Collection
	FollowingsCollection       *mongo.Collection
	PlacesCollection           *mongo.Collection
	SlotCollection             *mongo.Collection
	BookingsCollection         *mongo.Collection
	PostsCollection            *mongo.Collection
	FilesCollection            *mongo.Collection
	MerchCollection            *mongo.Collection
	MenuCollection             *mongo.Collection
	ActivitiesCollection       *mongo.Collection
	EventsCollection           *mongo.Collection
	ArtistEventsCollection     *mongo.Collection
	SongsCollection            *mongo.Collection
	SearchCollection           *mongo.Collection
	MediaCollection            *mongo.Collection
	ArtistsCollection          *mongo.Collection
	CartoonsCollection         *mongo.Collection
	ChatsCollection            *mongo.Collection
	MessagesCollection         *mongo.Collection
	Client                     *mongo.Client
	CTX                        = context.Background()
)
