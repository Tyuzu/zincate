package structs

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Event struct {
	EventID           string                 `json:"eventid" bson:"eventid"`
	Title             string                 `json:"title" bson:"title"`
	Description       string                 `json:"description" bson:"description"`
	Place             string                 `json:"place" bson:"place"`
	Date              string                 `json:"date" bson:"date"`
	Location          string                 `json:"location" bson:"location"`
	CreatorID         string                 `json:"creatorid" bson:"creatorid"`
	OrganizerName     string                 `json:"organizer_name" bson:"organizer_name"`
	OrganizerContact  string                 `json:"organizer_contact" bson:"organizer_contact"`
	Tickets           []Ticket               `json:"tickets" bson:"tickets"`
	Merch             []Merch                `json:"merch" bson:"merch"`
	StartDateTime     time.Time              `json:"start_date_time" bson:"start_date_time"`
	EndDateTime       time.Time              `json:"end_date_time" bson:"end_date_time"`
	Category          string                 `json:"category" bson:"category"`
	BannerImage       string                 `json:"banner_image" bson:"banner_image"`
	WebsiteURL        string                 `json:"website_url" bson:"website_url"`
	Status            string                 `json:"status" bson:"status"`
	AccessibilityInfo string                 `json:"accessibility_info" bson:"accessibility_info"`
	SocialMediaLinks  []string               `json:"social_links" bson:"social_links"`
	Tags              []string               `json:"tags" bson:"tags"`
	CustomFields      map[string]interface{} `json:"custom_fields" bson:"custom_fields"`
	CreatedAt         time.Time              `json:"created_at" bson:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at" bson:"updated_at"`
}

type Tag struct {
	ID     string   `json:"id,omitempty" bson:"_id,omitempty"`
	Name   string   `json:"name,omitempty" bson:"name,omitempty"`
	Places []string `json:"places,omitempty" bson:"places,omitempty"` // List of Place IDs tagged with this keyword
}

type Merch struct {
	MerchID     string             `json:"merchid" bson:"merchid"`
	EventID     string             `json:"eventid" bson:"eventid"` // Reference to Event ID
	Name        string             `json:"name" bson:"name"`
	Price       float64            `json:"price" bson:"price"`
	Stock       int                `json:"stock" bson:"stock"` // Number of items available
	MerchPhoto  string             `json:"merch_pic" bson:"merch_pic"`
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	EntityID    primitive.ObjectID `json:"entity_id" bson:"entity_id"`
	EntityType  string             `json:"entity_type" bson:"entity_type"` // "event" or "place"
	Description string             `json:"description,omitempty" bson:"description,omitempty"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	UserID      primitive.ObjectID `bson:"user_id" json:"userId"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updatedAt"`
}

type Ticket struct {
	TicketID    string             `json:"ticketid" bson:"ticketid"`
	EventID     string             `json:"eventid" bson:"eventid"`
	Name        string             `json:"name" bson:"name"`
	Price       float64            `json:"price" bson:"price"`
	Quantity    int                `json:"quantity" bson:"quantity"`
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	EntityID    primitive.ObjectID `json:"entity_id" bson:"entity_id"`
	EntityType  string             `json:"entity_type" bson:"entity_type"` // "event" or "place"
	Available   int                `json:"available" bson:"available"`
	Total       int                `json:"total" bson:"total"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	Description string             `bson:"description,omitempty" json:"description"`
	Sold        int                `bson:"sold" json:"sold"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updatedAt"`
}
