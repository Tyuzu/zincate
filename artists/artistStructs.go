package artists

import "go.mongodb.org/mongo-driver/bson/primitive"

type Song struct {
	SongID      string             `json:"songid" bson:"songid,omitempty"`
	ArtistID    string             `json:"artistid" bson:"artistid,omitempty"`
	Title       string             `json:"title" bson:"title"`
	Genre       string             `json:"genre" bson:"genre"`
	Duration    string             `json:"duration" bson:"duration"`
	Description string             `json:"description,omitempty" bson:"description,omitempty"`
	AudioURL    string             `json:"audioUrl,omitempty" bson:"audioUrl,omitempty"`
	Published   bool               `json:"published" bson:"published"`
	Plays       int                `json:"plays,omitempty" bson:"plays,omitempty"`
	UploadedAt  primitive.DateTime `json:"uploadedAt" bson:"uploadedAt"`
	Poster      string             `bson:"poster,omitempty" json:"poster,omitempty"`
}

type Album struct {
	Title       string `json:"title"`
	ReleaseDate string `json:"releaseDate"`
	Description string `json:"description"`
	Published   bool   `json:"published"`
}

type Post struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
	Published bool   `json:"published"`
}

type MerchItem struct {
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
	Image       string  `json:"image,omitempty"`
	Visible     bool    `json:"visible"`
	MerchID     string  `json:"merchid" bson:"merchid"`
}

type Event struct {
	Title     string `json:"title"`
	Date      string `json:"date"`
	Venue     string `json:"venue"`
	City      string `json:"city"`
	Country   string `json:"country"`
	TicketURL string `json:"ticketUrl,omitempty"`
}
