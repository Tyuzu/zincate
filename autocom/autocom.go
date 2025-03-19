package autocom

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

// Load environment variables and initialize Redis connection
func InitRedis() *redis.Client {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found, using system environment variables")
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Fatal("REDIS_URL is not set")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: os.Getenv("REDIS_PASSWORD"), // Empty if no password
		DB:       0,                           // Default DB
	})

	return client
}

// Get a Redis client instance
func GetRedisClient() *redis.Client {
	return InitRedis()
}

// Add an event for autocorrect suggestions
func AddEventToAutocorrect(client *redis.Client, eventID, eventName string) error {
	ctx := context.Background()
	key := "autocomplete:events"

	_, err := client.ZAdd(ctx, key, []redis.Z{
		{
			Score:  0,         // Lower scores appear first (you can modify this logic)
			Member: eventName, // Store event name for autocomplete
		},
	}...).Result()

	if err != nil {
		return fmt.Errorf("failed to add event to autocomplete: %v", err)
	}

	log.Printf("Event added for autocorrect: %s", eventName)
	return nil
}

// Add a place for autocorrect suggestions
func AddPlaceToAutocorrect(client *redis.Client, placeID, placeName string) error {
	ctx := context.Background()
	key := "autocomplete:places"

	_, err := client.ZAdd(ctx, key, []redis.Z{
		{
			Score:  0,
			Member: placeName,
		},
	}...).Result()

	if err != nil {
		return fmt.Errorf("failed to add place to autocomplete: %v", err)
	}

	log.Printf("Place added for autocorrect: %s", placeName)
	return nil
}

// Search event suggestions based on user input
func SearchEventAutocorrect(client *redis.Client, query string, limit int64) ([]string, error) {
	ctx := context.Background()
	key := "autocomplete:events"

	// Get matching event names
	results, err := client.ZRangeByLex(ctx, key, &redis.ZRangeBy{
		Min:    "[" + query,
		Max:    "[" + query + "\xff",
		Offset: 0,
		Count:  limit,
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to search events in autocomplete: %v", err)
	}

	return results, nil
}

// Search place suggestions based on user input
func SearchPlaceAutocorrect(client *redis.Client, query string, limit int64) ([]string, error) {
	ctx := context.Background()
	key := "autocomplete:places"

	// Get matching place names
	results, err := client.ZRangeByLex(ctx, key, &redis.ZRangeBy{
		Min:    "[" + query,
		Max:    "[" + query + "\xff",
		Offset: 0,
		Count:  limit,
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to search places in autocomplete: %v", err)
	}

	return results, nil
}
