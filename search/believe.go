package search

import (
	"encoding/json"
	"fmt"
	"log"
	"naevis/db"
	"naevis/models"
	"naevis/rdx"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Assuming ctx is defined either here or imported from your init package.
// var ctx = context.Background()

// -------------------------
// TYPES & UTILITY FUNCTIONS
// -------------------------

// Entity represents an indexable item.
type Entity struct {
	ID          string    `json:"id" bson:"_id"`    // using _id as the primary key in MongoDB
	Type        string    `json:"type" bson:"type"` // e.g., "event", "post", "media"
	Title       string    `json:"title" bson:"title"`
	Description string    `json:"description" bson:"description"`
	CreatedAt   time.Time `json:"createdAt" bson:"createdAt"`
}

// TrieNode supports autocomplete (if needed).
type TrieNode struct {
	Children map[rune]*TrieNode
	IsWord   bool
}

func NewTrieNode() *TrieNode {
	return &TrieNode{Children: make(map[rune]*TrieNode)}
}

// Global Trie root.
var TrieRoot = NewTrieNode()

// Stop words for tokenization.
var stopWords = map[string]bool{
	"the": true, "and": true, "of": true, "in": true, "to": true,
	"for": true, "on": true, "with": true, "0": true, "1": true, "2": true, "3": true, "4": true, "5": true, "6": true, "7": true, "8": true, "9": true, "*": true,
}

// Tokenize converts text to lower-case, splits it, and removes punctuation and stop words.
func Tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.Fields(text)
	var tokens []string
	for _, word := range words {
		word = strings.Trim(word, ".,!?")
		if !stopWords[word] {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

// ExtractHashtags returns words starting with "#".
func ExtractHashtags(text string) []string {
	words := strings.Fields(text)
	var hashtags []string
	for _, word := range words {
		if strings.HasPrefix(word, "#") {
			hashtags = append(hashtags, word)
		}
	}
	return hashtags
}

// -------------------------
// REDIS PERSISTENCE FUNCTIONS
// -------------------------

// AddToIndex adds an entity to a Redis sorted set using the given key.
// For the inverted index, the key is "inverted:<token>".
// For hashtags, we use "hashtag:<token>".
func AddToIndex(indexKey, entityID string, createdAt time.Time) error {
	score := float64(createdAt.UnixNano())
	return rdx.Conn.ZAdd(ctx, indexKey, redis.Z{
		Score:  score,
		Member: entityID,
	}).Err()
}

// DeleteFromIndex removes an entity from a Redis sorted set using the given key.
func DeleteFromIndex(indexKey, entityID string) error {
	return rdx.Conn.ZRem(ctx, indexKey, entityID).Err()
}

// GetIndex returns all members (with scores) from a Redis sorted set.
func GetIndex(indexKey string) ([]redis.Z, error) {
	// Retrieve all elements in descending order (newest first).
	return rdx.Conn.ZRevRangeWithScores(ctx, indexKey, 0, -1).Result()
}

// CacheSearchResult stores search results (as JSON) with an expiration.
func CacheSearchResult(key string, result []string) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return rdx.Conn.Set(ctx, key, data, time.Hour).Err()
}

// GetCachedSearchResult retrieves a cached search result.
func GetCachedSearchResult(key string) ([]string, error) {
	data, err := rdx.Conn.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var result []string
	if err = json.Unmarshal([]byte(data), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// -------------------------
// INDEXING FUNCTIONS
// -------------------------

// GetResultsByType fetches results based on entity type using a reverse index.
// If the primary fetch fails, it attempts to get the entity from the search collection.
func GetResultsByType(entityType string, id string) any {
	log.Println("GetResultsByType:")
	log.Println("entityType:", entityType)
	log.Println("id:", id)

	var result any

	switch entityType {
	case "event":
		filter := bson.M{"eventid": id}
		var ev models.Event
		err := FetchAndDecode("events", filter, &ev)
		if err != nil || ev.EventID == "" {
			log.Println("Error fetching event from primary collection, error:", err)
			result = FetchEntityFromSearchDB(id)
		} else {
			log.Println("ev:", ev)
			result = ev
		}
	case "place":
		filter := bson.M{"placeid": id}
		var p models.Place
		err := FetchAndDecode("places", filter, &p)
		if err != nil || p.PlaceID == "" {
			log.Println("Error fetching place from primary collection, error:", err)
			result = FetchEntityFromSearchDB(id)
		} else {
			log.Println("p:", p)
			result = p
		}
	default:
		return nil
	}

	return result
}

// ConvertToEntity transforms various entity types into a standard Entity struct.
// It then saves the entity to the search collection.
func ConvertToEntity(data any) (Entity, error) {
	var entity Entity

	switch v := data.(type) {
	case models.Event:
		entity.ID = v.EventID
		entity.Title = v.Title
		entity.Description = v.Description
		entity.Type = "event"
		entity.CreatedAt = parseTime(v.Date)
	case models.Place:
		entity.ID = v.PlaceID
		entity.Title = v.Name
		entity.Description = v.Description
		entity.Type = "place"
		entity.CreatedAt = parseTime(v.CreatedAt)
	case Entity:
		// In case we already have an Entity from the search collection.
		entity = v
	default:
		return Entity{}, fmt.Errorf("unsupported entity type")
	}

	if err := SaveEntityToDB(entity); err != nil {
		log.Println("Warning: could not save entity to search collection:", err)
	}
	return entity, nil
}

// parseTime converts various time formats into a standard time.Time value.
func parseTime(value any) time.Time {
	switch v := value.(type) {
	case int:
		return time.Unix(0, int64(v)*int64(time.Millisecond))
	case int32:
		return time.Unix(0, int64(v)*int64(time.Millisecond))
	case int64:
		return time.Unix(0, v*int64(time.Millisecond))
	case float64:
		return time.Unix(0, int64(v)*int64(time.Millisecond))
	case string:
		if millis, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Unix(0, millis*int64(time.Millisecond))
		}
	}
	return time.Now()
}

// SaveEntityToDB saves the provided Entity into a dedicated search collection.
func SaveEntityToDB(entity Entity) error {
	filter := bson.M{"_id": entity.ID}
	update := bson.M{"$set": entity}
	opts := options.Update().SetUpsert(true)
	_, err := db.SearchCollection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save entity to search DB: %w", err)
	}
	return nil
}

// FetchEntityFromSearchDB attempts to retrieve an Entity from the search collection.
func FetchEntityFromSearchDB(id string) Entity {
	filter := bson.M{"_id": id}
	var entity Entity
	err := db.SearchCollection.FindOne(ctx, filter).Decode(&entity)
	if err != nil {
		log.Println("Error fetching entity from search DB:", err)
	}
	return entity
}

// DeleteEntity removes the entity from all indexes.
func DeleteEntity(event models.Index) error {
	// First, try to fetch the entity using GetResultsByType.
	data := GetResultsByType(event.EntityType, event.EntityId)
	if data == nil {
		return fmt.Errorf("no data found for deletion for %s with ID %s", event.EntityType, event.EntityId)
	}
	// Convert the data to our generic Entity struct.
	entity, err := ConvertToEntity(data)
	if err != nil {
		return fmt.Errorf("error converting data to entity for deletion: %v", err)
	}

	// Tokenize the text used during indexing.
	text := strings.ToLower(entity.Title + " " + entity.Description)
	tokens := Tokenize(text)

	// Remove entity from each token's index.
	for _, token := range tokens {
		invKey := fmt.Sprintf("inverted:%s", token)
		if err := DeleteFromIndex(invKey, entity.ID); err != nil {
			return fmt.Errorf("error deleting from inverted index: %v", err)
		}
		// Also remove from hashtag index if token is a hashtag.
		if strings.HasPrefix(token, "#") {
			hashKey := fmt.Sprintf("hashtag:%s", token)
			if err := DeleteFromIndex(hashKey, entity.ID); err != nil {
				return fmt.Errorf("error deleting from hashtag index: %v", err)
			}
		}
	}
	return nil
}

// IndexEntity persists the entity and updates indexes (inverted, hashtag, autocomplete).
func IndexEntity(entity Entity) error {
	// Index tokens.
	text := strings.ToLower(entity.Title + " " + entity.Description)
	tokens := Tokenize(text)
	for _, token := range tokens {
		invKey := fmt.Sprintf("inverted:%s", token)
		if err := AddToIndex(invKey, entity.ID, entity.CreatedAt); err != nil {
			return err
		}
		// For hashtags, use a dedicated index.
		if strings.HasPrefix(token, "#") {
			hashKey := fmt.Sprintf("hashtag:%s", token)
			if err := AddToIndex(hashKey, entity.ID, entity.CreatedAt); err != nil {
				return err
			}
		}
		// For autocomplete, add all substrings.
		for i := 0; i < len(token); i++ {
			TrieRoot.AddWord(token[i:])
		}
	}
	return nil
}

func IndexDatainRedis(event models.Index) error {
	switch event.Method {
	case "DELETE":
		return DeleteEntity(event)
	case "PUT":
		// Get the latest data (e.g., from the primary source or provided update payload)
		data := GetResultsByType(event.EntityType, event.EntityId)
		if data == nil {
			return fmt.Errorf("no data found for %s with ID %s", event.EntityType, event.EntityId)
		}
		// Convert the data to an Entity format.
		newEntity, err := ConvertToEntity(data)
		if err != nil {
			return fmt.Errorf("error converting data to entity: %v", err)
		}
		// Update indexes based on the changes.
		return UpdateEntityIndexes(newEntity)
	default:
		// Handle insert (or default case) as before.
		data := GetResultsByType(event.EntityType, event.EntityId)
		if data == nil {
			return fmt.Errorf("no data found for %s with ID %s", event.EntityType, event.EntityId)
		}
		entity, err := ConvertToEntity(data)
		if err != nil {
			return fmt.Errorf("error converting data to entity: %v", err)
		}
		return IndexEntity(entity)
	}
}

// UpdateEntityIndexes updates indexes for an updated entity.
func UpdateEntityIndexes(newEntity Entity) error {
	// 1. Fetch the old entity from the search collection.
	oldEntity := FetchEntityFromSearchDB(newEntity.ID)
	if oldEntity.ID == "" {
		// If no previous version exists, simply add the new entity.
		return IndexEntity(newEntity)
	}

	// 2. Tokenize both the old and new texts.
	oldText := strings.ToLower(oldEntity.Title + " " + oldEntity.Description)
	newText := strings.ToLower(newEntity.Title + " " + newEntity.Description)
	oldTokens := make(map[string]bool)
	newTokens := make(map[string]bool)

	for _, token := range Tokenize(oldText) {
		oldTokens[token] = true
	}
	for _, token := range Tokenize(newText) {
		newTokens[token] = true
	}

	// 3. Compute tokens to remove and tokens to add.
	var tokensToRemove []string
	var tokensToAdd []string

	// Tokens that exist in the old entity but not in the new entity.
	for token := range oldTokens {
		if !newTokens[token] {
			tokensToRemove = append(tokensToRemove, token)
		}
	}
	// Tokens that are new in the updated entity.
	for token := range newTokens {
		if !oldTokens[token] {
			tokensToAdd = append(tokensToAdd, token)
		}
	}

	// 4. Remove the entity from indexes for removed tokens.
	for _, token := range tokensToRemove {
		invKey := fmt.Sprintf("inverted:%s", token)
		if err := DeleteFromIndex(invKey, newEntity.ID); err != nil {
			return fmt.Errorf("error deleting from inverted index for token %s: %v", token, err)
		}
		if strings.HasPrefix(token, "#") {
			hashKey := fmt.Sprintf("hashtag:%s", token)
			if err := DeleteFromIndex(hashKey, newEntity.ID); err != nil {
				return fmt.Errorf("error deleting from hashtag index for token %s: %v", token, err)
			}
		}
	}

	// 5. Add the entity to indexes for newly added tokens.
	for _, token := range tokensToAdd {
		invKey := fmt.Sprintf("inverted:%s", token)
		if err := AddToIndex(invKey, newEntity.ID, newEntity.CreatedAt); err != nil {
			return fmt.Errorf("error adding to inverted index for token %s: %v", token, err)
		}
		if strings.HasPrefix(token, "#") {
			hashKey := fmt.Sprintf("hashtag:%s", token)
			if err := AddToIndex(hashKey, newEntity.ID, newEntity.CreatedAt); err != nil {
				return fmt.Errorf("error adding to hashtag index for token %s: %v", token, err)
			}
		}
		// For autocomplete, add all substrings.
		for i := 0; i < len(token); i++ {
			TrieRoot.AddWord(token[i:])
		}
	}

	// Optionally, if some tokens are common between old and new, you could update their score
	// if, for example, the CreatedAt timestamp has changed.

	// 6. Save the updated entity in the search collection.
	if err := SaveEntityToDB(newEntity); err != nil {
		return fmt.Errorf("failed to update entity in search DB: %v", err)
	}

	return nil
}

// ParallelIndexing processes a list of entities concurrently.
func ParallelIndexing(entities []Entity, numWorkers int) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(entities))
	entityChan := make(chan Entity, len(entities))

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entity := range entityChan {
				if err := IndexEntity(entity); err != nil {
					errChan <- err
				}
			}
		}()
	}

	for _, entity := range entities {
		entityChan <- entity
	}
	close(entityChan)
	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return <-errChan
	}
	return nil
}
