package search

import (
	"go.mongodb.org/mongo-driver/bson"
)

// Global per-entity projections.
var Projections = map[string]bson.M{
	"event": {
		"eventid":      1,
		"title":        1,
		"location":     1,
		"date":         1,
		"category":     1,
		"price":        1,
		"description":  1,
		"contact":      1,
		"banner_image": 1,
	},
	"ticket": {
		"ticketid": 1,
		"eventid":  1,
		"price":    1,
		"date":     1,
		"link":     1,
	},
	"merch": {
		"merchid":     1,
		"name":        1,
		"price":       1,
		"description": 1,
		"image":       1,
	},
	"media": {
		"entityid":   1,
		"entitytype": 1,
		"id":         1,
		"media_url":  1,
	},
	"review": {
		"reviewid": 1,
		"userid":   1,
		"rating":   1,
		"comment":  1,
		"date":     1,
	},
	"place": {
		"placeid":     1,
		"name":        1,
		"location":    1,
		"category":    1,
		"contact":     1,
		"banner":      1,
		"description": 1,
	},
	"menu": {
		"menuid":      1,
		"placeid":     1,
		"name":        1,
		"description": 1,
	},
	"feedpost": {
		"postid":   1,
		"userid":   1,
		"username": 1,
		"content":  1,
		"date":     1,
	},
	"profile": {
		"userid":   1,
		"username": 1,
		"email":    1,
		"contact":  1,
		"image":    1,
	},
	// Add other entity types as needed.
}
