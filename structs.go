package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserFollow struct {
	UserID    string   `json:"userid" bson:"userid"`
	Follows   []string `json:"follows,omitempty" bson:"follows,omitempty"`
	Followers []string `json:"followers,omitempty" bson:"followers,omitempty"`
	// Follows   []primitive.ObjectID `json:"follows,omitempty" bson:"follows,omitempty"`
	// Followers []primitive.ObjectID `json:"followers,omitempty" bson:"followers,omitempty"`
}

// UserProfileResponse defines the structure for the user profile response
type UserProfileResponse struct {
	UserID         string            `json:"userid" bson:"userid"`
	Username       string            `json:"username" bson:"username"`
	Email          string            `json:"email" bson:"email"`
	Bio            string            `json:"bio,omitempty" bson:"bio,omitempty"`
	PhoneNumber    string            `json:"phone_number,omitempty" bson:"phone_number,omitempty"`
	ProfilePicture string            `json:"profile_picture" bson:"profile_picture"`
	BannerPicture  string            `json:"banner_picture" bson:"banner_picture"`
	IsFollowing    bool              `json:"is_following" bson:"is_following"` // Added here
	Followers      int               `json:"followers" bson:"followers"`
	Follows        int               `json:"follows" bson:"follows"`
	SocialLinks    map[string]string `json:"social_links,omitempty" bson:"social_links,omitempty"`
}
type Activity struct {
	Username     string              `json:"username,omitempty" bson:"username,omitempty"`
	PlaceID      string              `json:"placeId,omitempty" bson:"placeId,omitempty"`
	Action       string              `json:"action,omitempty" bson:"action,omitempty"`
	PerformedBy  string              `json:"performedBy,omitempty" bson:"performedBy,omitempty"`
	Timestamp    time.Time           `json:"timestamp,omitempty" bson:"timestamp,omitempty"`
	Details      string              `json:"details,omitempty" bson:"details,omitempty"`
	IPAddress    string              `json:"ipAddress,omitempty" bson:"ipAddress,omitempty"`
	DeviceInfo   string              `json:"deviceInfo,omitempty" bson:"deviceInfo,omitempty"`
	ID           primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	UserID       primitive.ObjectID  `json:"user_id" bson:"user_id"`
	ActivityType string              `json:"activity_type" bson:"activity_type"` // e.g., "follow", "review", "buy"
	EntityID     *primitive.ObjectID `json:"entity_id,omitempty" bson:"entity_id,omitempty"`
	EntityType   *string             `json:"entity_type,omitempty" bson:"entity_type,omitempty"` // "event", "place", or null
}

type User struct {
	// ID          string    `json:"-" bson:"_id,omitempty"`
	UserID         string            `json:"userid" bson:"userid"`
	Username       string            `json:"username" bson:"username"`
	Email          string            `json:"email" bson:"email"`
	Password       string            `json:"-" bson:"password"`
	Role           string            `json:"role" bson:"role"`
	Name           string            `json:"name,omitempty" bson:"name,omitempty"`
	CreatedAt      time.Time         `json:"created_at" bson:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at" bson:"updated_at"`
	PhoneNumber    string            `json:"phone_number,omitempty" bson:"phone_number,omitempty"`
	Bio            string            `json:"bio,omitempty" bson:"bio,omitempty"`
	IsActive       bool              `json:"is_active" bson:"is_active"`
	LastLogin      time.Time         `json:"last_login,omitempty" bson:"last_login,omitempty"`
	ProfilePicture string            `json:"profile_picture" bson:"profile_picture"`
	BannerPicture  string            `json:"banner_picture" bson:"banner_picture"`
	ProfileViews   int               `json:"profile_views,omitempty" bson:"profile_views,omitempty"`
	Address        string            `json:"address,omitempty" bson:"address,omitempty"`
	DateOfBirth    time.Time         `json:"date_of_birth,omitempty" bson:"date_of_birth,omitempty"`
	SocialLinks    map[string]string `json:"social_links,omitempty" bson:"social_links,omitempty"`
	IsVerified     bool              `json:"is_verified" bson:"is_verified"`
	// Follows        []string             `json:"follows,omitempty" bson:"follows,omitempty"`
	// Followers      []string             `json:"followers,omitempty" bson:"followers,omitempty"`
	PasswordHash string               `json:"password_hash" bson:"password_hash"`
	Banner       string               `json:"banner,omitempty" bson:"banner,omitempty"`
	Following    []primitive.ObjectID `json:"following" bson:"following"`
}

// type User struct {
// 	// ID          string    `json:"-" bson:"_id,omitempty"`
// 	UserID       string    `json:"userid" bson:"userid"`
// 	Username     string    `json:"username" bson:"username"`
// 	Email        string    `json:"email" bson:"email"`
// 	Password     string    `json:"-" bson:"password"`
// 	Role         string    `json:"role" bson:"role"`
// 	Name         string    `json:"name,omitempty" bson:"name,omitempty"`
// 	CreatedAt    time.Time `json:"created_at" bson:"created_at"`
// 	UpdatedAt    time.Time `json:"updated_at" bson:"updated_at"`
// 	PhoneNumber  string    `json:"phone_number,omitempty" bson:"phone_number,omitempty"`
// 	IsActive     bool      `json:"is_active" bson:"is_active"`
// 	LastLogin    time.Time `json:"last_login,omitempty" bson:"last_login,omitempty"`
// 	ProfileViews int       `json:"profile_views,omitempty" bson:"profile_views,omitempty"`
// 	Address      string    `json:"address,omitempty" bson:"address,omitempty"`
// 	DateOfBirth  time.Time `json:"date_of_birth,omitempty" bson:"date_of_birth,omitempty"`
// 	IsVerified   bool      `json:"is_verified" bson:"is_verified"`
// 	PasswordHash string    `json:"password_hash" bson:"password_hash"`
// }

type Response struct {
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type Setting struct {
	Type        string      `json:"type"`
	Value       interface{} `json:"value"`
	Description string      `json:"description"`
}

type UserSettings struct {
	UserID   string    `bson:"userID" json:"userID"`
	Settings []Setting `bson:"settings" json:"settings"`
}

type Post struct {
	ID        interface{} `bson:"_id,omitempty" json:"id"`
	UserID    string      `json:"userid" bson:"userid"`
	Username  string      `bson:"username" json:"username"`
	Text      string      `bson:"text" json:"text"`
	Type      string      `bson:"type" json:"type"`   // Post type (e.g., "text", "image", "video", "blog", etc.)
	Media     []string    `bson:"media" json:"media"` // Media URLs (images, videos, etc.)
	Timestamp string      `bson:"timestamp" json:"timestamp"`
	Likes     int         `bson:"likes" json:"likes"`
	// ID         primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	Content   string               `json:"content" bson:"content"`
	MediaURL  string               `json:"media_url,omitempty" bson:"media_url,omitempty"`
	Likers    []primitive.ObjectID `json:"likers" bson:"likers"`
	CreatedAt time.Time            `json:"created_at" bson:"created_at"`
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

type Seat struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	EntityID   primitive.ObjectID `json:"entity_id" bson:"entity_id"`
	EntityType string             `json:"entity_type" bson:"entity_type"` // e.g., "event" or "place"
	SeatNumber string             `json:"seat_number" bson:"seat_number"`
	UserID     primitive.ObjectID `json:"user_id" bson:"user_id,omitempty"`
	Status     string             `json:"status" bson:"status"` // e.g., "booked", "available"
}

// UserProfileResponse defines the structure for the user profile response
type UserSuggest struct {
	UserID         string `json:"userid" bson:"userid"`
	Username       string `json:"username" bson:"username"`
	Bio            string `json:"bio,omitempty" bson:"bio,omitempty"`
	ProfilePicture string `json:"profile_picture" bson:"profile_picture"`
	IsFollowing    bool   `json:"is_following" bson:"is_following"` // Added here
}

type Suggestion struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Type        string             `json:"type" bson:"type"` // e.g., "place" or "event"
	Title       string             `json:"title" bson:"title"`
	Description string             `json:"description,omitempty" bson:"description,omitempty"`
	Name        string             `json:"name"`
}

type Review struct {
	EntityID    string    `json:"entity_id" bson:"entity_id"`
	EntityType  string    `json:"entity_type" bson:"entity_type"` // "event" or "place"
	Comment     string    `json:"comment,omitempty" bson:"comment,omitempty"`
	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
	Content     string    `bson:"content" json:"content"`
	ReviewID    string    `json:"reviewid" bson:"reviewid"`
	UserID      string    `json:"userid" bson:"userid"` // Reference to User ID
	Rating      int       `json:"rating" bson:"rating"` // Rating out of 5
	Date        time.Time `json:"date" bson:"date"`     // Date of the review
	Likes       int       `json:"likes,omitempty" bson:"likes,omitempty"`
	Dislikes    int       `json:"dislikes,omitempty" bson:"dislikes,omitempty"`
	Attachments []string  `json:"attachments,omitempty" bson:"attachments,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
}

type Media struct {
	ID            string             `json:"id" bson:"id"`
	Type          string             `json:"type" bson:"type"`
	URL           string             `json:"url" bson:"url"`
	ThumbnailURL  string             `json:"thumbnailUrl,omitempty" bson:"thumbnailUrl,omitempty"`
	Caption       string             `json:"caption" bson:"caption"`
	Description   string             `json:"description,omitempty" bson:"description,omitempty"`
	CreatorID     string             `json:"creatorid" bson:"creatorid"`
	LikesCount    int                `json:"likesCount" bson:"likesCount"`
	CommentsCount int                `json:"commentsCount" bson:"commentsCount"`
	Visibility    string             `json:"visibility" bson:"visibility"`
	Tags          []string           `json:"tags,omitempty" bson:"tags,omitempty"`
	Duration      int                `json:"duration,omitempty" bson:"duration,omitempty"`
	FileSize      int64              `json:"fileSize,omitempty" bson:"fileSize,omitempty"`
	MimeType      string             `json:"mimeType,omitempty" bson:"mimeType,omitempty"`
	IsFeatured    bool               `json:"isFeatured,omitempty" bson:"isFeatured,omitempty"`
	EntityID      string             `json:"entityid" bson:"entityid"`
	EntityType    string             `json:"entitytype" bson:"entitytype"` // "event" or "place""video"
	CreatedAt     time.Time          `json:"created_at" bson:"created_at"`
	UserID        primitive.ObjectID `bson:"user_id" json:"userId"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updatedAt"`
}

type Place struct {
	PlaceID           string                 `json:"placeid" bson:"placeid"`
	Name              string                 `json:"name" bson:"name"`
	Description       string                 `json:"description" bson:"description"`
	Place             string                 `json:"place" bson:"place"`
	Capacity          int                    `json:"capacity" bson:"capacity"`
	Date              string                 `json:"date" bson:"date"`
	Address           string                 `json:"address" bson:"address"`
	CreatedBy         string                 `json:"createdBy,omitempty" bson:"createdBy,omitempty"`
	OrganizerName     string                 `json:"organizer_name" bson:"organizer_name"`
	OrganizerContact  string                 `json:"organizer_contact" bson:"organizer_contact"`
	Tickets           []Ticket               `json:"tickets" bson:"tickets"`
	Merch             []Merch                `json:"merch" bson:"merch"`
	StartDateTime     time.Time              `json:"start_date_time" bson:"start_date_time"`
	EndDateTime       time.Time              `json:"end_date_time" bson:"end_date_time"`
	Category          string                 `json:"category" bson:"category"`
	Banner            string                 `json:"banner" bson:"banner"`
	WebsiteURL        string                 `json:"website_url" bson:"website_url"`
	Status            string                 `json:"status" bson:"status"`
	AccessibilityInfo string                 `json:"accessibility_info" bson:"accessibility_info"`
	SocialMediaLinks  []string               `json:"social_links" bson:"social_links"`
	Tags              []string               `json:"tags" bson:"tags"`
	CustomFields      map[string]interface{} `json:"custom_fields" bson:"custom_fields"`
	CreatedAt         time.Time              `json:"created_at" bson:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at" bson:"updated_at"`
	City              string                 `json:"city,omitempty" bson:"city,omitempty"`
	Country           string                 `json:"country,omitempty" bson:"country,omitempty"`
	ZipCode           string                 `json:"zipCode,omitempty" bson:"zipCode,omitempty"`
	Coordinates       Coordinates            `json:"coordinates,omitempty" bson:"coordinates,omitempty"`
	Phone             string                 `json:"phone,omitempty" bson:"phone,omitempty"`
	Website           string                 `json:"website,omitempty" bson:"website,omitempty"`
	IsOpen            bool                   `json:"isopen,omitempty" bson:"isopen,omitempty"`
	Distance          float64                `json:"distance,omitempty" bson:"distance,omitempty"`
	Views             int                    `json:"views,omitempty" bson:"views,omitempty"`
	ReviewCount       int                    `json:"reviewCount,omitempty" bson:"reviewCount,omitempty"`
	SocialLinks       map[string]string      `json:"socialLinks,omitempty" bson:"socialLinks,omitempty"`
	UpdatedBy         string                 `json:"updatedBy,omitempty" bson:"updatedBy,omitempty"`
	DeletedAt         *time.Time             `json:"deletedAt,omitempty" bson:"deletedAt,omitempty"`
	Amenities         []string               `json:"amenities,omitempty" bson:"amenities,omitempty"`
	Events            []string               `json:"events,omitempty" bson:"events,omitempty"`
	OperatingHours    []string               `json:"operatinghours,omitempty" bson:"operatinghours,omitempty"`
	Keywords          []string               `json:"keywords,omitempty" bson:"keywords,omitempty"`
}

type PlaceStatus string

const (
	Active   PlaceStatus = "active"
	Inactive PlaceStatus = "inactive"
	Closed   PlaceStatus = "closed"
)

type Coordinates struct {
	Latitude  float64 `json:"latitude,omitempty" bson:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty" bson:"longitude,omitempty"`
}

type CheckIn struct {
	UserID    string    `json:"userId,omitempty" bson:"userId,omitempty"`
	PlaceID   string    `json:"placeId,omitempty" bson:"placeId,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty" bson:"timestamp,omitempty"`
	Comment   string    `json:"comment,omitempty" bson:"comment,omitempty"`
	Rating    float64   `json:"rating,omitempty" bson:"rating,omitempty"` // Optional
	Medias    []Media   `json:"images,omitempty" bson:"images,omitempty"` // Optional
}

type PlaceVersion struct {
	PlaceID   string            `json:"placeId,omitempty" bson:"placeId,omitempty"`
	Version   int               `json:"version,omitempty" bson:"version,omitempty"`
	Data      Place             `json:"data,omitempty" bson:"data,omitempty"`
	UpdatedAt time.Time         `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
	UpdatedBy string            `json:"updatedBy,omitempty" bson:"updatedBy,omitempty"`
	Changes   map[string]string `json:"changes,omitempty" bson:"changes,omitempty"`
}

type OperatingHours struct {
	Day          []string `json:"day,omitempty" bson:"day,omitempty"`
	OpeningHours []string `json:"opening,omitempty" bson:"opening,omitempty"`
	ClosingHours []string `json:"closing,omitempty" bson:"closing,omitempty"`
	TimeZone     string   `json:"timeZone,omitempty" bson:"timeZone,omitempty"`
}

type Tag struct {
	ID     string   `json:"id,omitempty" bson:"_id,omitempty"`
	Name   string   `json:"name,omitempty" bson:"name,omitempty"`
	Places []string `json:"places,omitempty" bson:"places,omitempty"` // List of Place IDs tagged with this keyword
}

const (
	PlaceStatusActive     = "active"
	PlaceStatusClosed     = "closed"
	PlaceStatusRenovation = "under renovation"
)

const (
	MediaTypeImage    = "image"
	MediaTypeVideo    = "video"
	MediaTypePhoto360 = "photo360"
)

type Business struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name        string             `json:"name" bson:"name"`
	Type        string             `json:"type" bson:"type"`
	Location    string             `json:"location" bson:"location"`
	Description string             `json:"description" bson:"description"`
}

type Booking struct {
	ID         primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	BusinessID primitive.ObjectID `json:"business_id" bson:"business_id"`
	UserName   string             `json:"user_name" bson:"user_name"`
	TimeSlot   string             `json:"time_slot" bson:"time_slot"`
}

type MenuItem struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name        string             `json:"name" bson:"name"`
	Price       float64            `json:"price" bson:"price"`
	Description string             `json:"description" bson:"description"`
}

type Promotion struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Title       string             `json:"title" bson:"title"`
	Description string             `json:"description" bson:"description"`
	ExpiryDate  time.Time          `json:"expiry_date" bson:"expiry_date"`
}

// Owner Management Handlers
type Owner struct {
	ID       primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name     string             `json:"name" bson:"name"`
	Email    string             `json:"email" bson:"email"`
	Password string             `json:"password" bson:"password"`
}