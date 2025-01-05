package main

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// User-related data access functions

func GetUserByUsername(username string) (*User, error) {
	var user User
	err := userCollection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // User not found
		}
		return nil, err
	}
	return &user, nil
}

func UpdateUserByUsername(username string, update bson.M) error {
	_, err := userCollection.UpdateOne(
		context.TODO(),
		bson.M{"username": username},
		bson.M{"$set": update},
	)
	return err
}

func DeleteUserByID(userID string) error {
	_, err := userCollection.DeleteOne(context.TODO(), bson.M{"userid": userID})
	return err
}

// Follow-related data access functions

func GetUserFollowData(userID string) (*UserFollow, error) {
	var userFollow UserFollow
	err := followingsCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&userFollow)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // No follow data found
		}
		return nil, err
	}
	return &userFollow, nil
}
