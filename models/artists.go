package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Artist struct {
	// ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ArtistID string            `bson:"artistid,omitempty" json:"artistid"`
	Category string            `bson:"category" json:"category"`
	Name     string            `bson:"name" json:"name"`
	Place    string            `bson:"place" json:"place"`
	Country  string            `bson:"country" json:"country"`
	Bio      string            `bson:"bio" json:"bio"`
	DOB      string            `bson:"dob" json:"dob"`
	Photo    string            `bson:"photo" json:"photo"`
	Banner   string            `bson:"banner" json:"banner"`
	Genres   []string          `bson:"genres" json:"genres"`
	Socials  map[string]string `bson:"socials" json:"socials"`
	EventIDs []string          `bson:"events" json:"events"`
	Members  []BandMember      `bson:"members,omitempty" json:"members,omitempty"` // ✅ ADD THIS
}

type BandMember struct {
	Name string `bson:"name" json:"name"`
	Role string `bson:"role,omitempty" json:"role,omitempty"`
	DOB  string `bson:"dob,omitempty" json:"dob,omitempty"`
}

type Cartoon struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Category string             `bson:"category" json:"category"`
	Name     string             `bson:"name" json:"name"`
	Place    string             `bson:"place" json:"place"`
	Country  string             `bson:"country" json:"country"`
	Bio      string             `bson:"bio" json:"bio"`
	DOB      string             `bson:"dob" json:"dob"`
	Photo    string             `bson:"photo" json:"photo"`
	Banner   string             `bson:"banner" json:"banner"`
	Genres   []string           `bson:"genres" json:"genres"`
	Socials  map[string]string  `bson:"socials" json:"socials"`
	// Socials  []string `bson:"socials" json:"socials"`
	EventIDs []string `bson:"events" json:"events"`
}
