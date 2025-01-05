package repository

import (
	"context"
	"errors"

	"naevis/structs"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type EventRepository struct {
	Collection *mongo.Collection
	Tickets    *mongo.Collection
	Media      *mongo.Collection
	Merch      *mongo.Collection
}

func NewEventRepository(collection *mongo.Collection) *EventRepository {
	return &EventRepository{Collection: collection}
}

func (repo *EventRepository) GetEventWithDetails(ctx context.Context, eventID string) (*mongo.Cursor, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "eventid", Value: eventID}}}},
		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "ticks"},
			{Key: "localField", Value: "eventid"},
			{Key: "foreignField", Value: "eventid"},
			{Key: "as", Value: "tickets"},
		}}},
		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "merch"},
			{Key: "localField", Value: "eventid"},
			{Key: "foreignField", Value: "eventid"},
			{Key: "as", Value: "merch"},
		}}},
		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "reviews"},
			{Key: "let", Value: bson.D{
				{Key: "event_id", Value: "$eventid"},
			}},
			{Key: "pipeline", Value: mongo.Pipeline{
				bson.D{{Key: "$match", Value: bson.D{
					{Key: "$expr", Value: bson.D{
						{Key: "$and", Value: bson.A{
							bson.D{{Key: "$eq", Value: bson.A{"$entityid", "$$event_id"}}},
							bson.D{{Key: "$eq", Value: bson.A{"$entitytype", "event"}}},
						}},
					}},
				}}},
				bson.D{{Key: "$limit", Value: 10}},
				bson.D{{Key: "$skip", Value: 0}},
			}},
			{Key: "as", Value: "reviews"},
		}}},
	}

	return repo.Collection.Aggregate(ctx, pipeline)
}

func (repo *EventRepository) FindEventByID(ctx context.Context, eventID string) (*structs.Event, error) {
	var event structs.Event
	err := repo.Collection.FindOne(ctx, bson.M{"eventid": eventID}).Decode(&event)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &event, nil
}

func (repo *EventRepository) DeleteEvent(ctx context.Context, eventID string) error {
	_, err := repo.Collection.DeleteOne(ctx, bson.M{"eventid": eventID})
	return err
}

func (repo *EventRepository) DeleteRelatedData(ctx context.Context, eventID string) error {
	// Delete related tickets
	if _, err := repo.Tickets.DeleteMany(ctx, bson.M{"eventid": eventID}); err != nil {
		return errors.New("error deleting related tickets")
	}
	// Delete related media
	if _, err := repo.Media.DeleteMany(ctx, bson.M{"eventid": eventID}); err != nil {
		return errors.New("error deleting related media")
	}
	// Delete related merch
	if _, err := repo.Merch.DeleteMany(ctx, bson.M{"eventid": eventID}); err != nil {
		return errors.New("error deleting related merch")
	}
	return nil
}

func (repo *EventRepository) IsEventIDExists(ctx context.Context, eventID string) (bool, error) {
	err := repo.Collection.FindOne(ctx, bson.M{"eventid": eventID}).Err()
	if err == nil {
		return true, nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	return false, err
}

func (repo *EventRepository) InsertEvent(ctx context.Context, event structs.Event) error {
	_, err := repo.Collection.InsertOne(ctx, event)
	return err
}

func (repo *EventRepository) UpdateEvent(ctx context.Context, eventID string, updateFields bson.M) (int64, error) {
	result, err := repo.Collection.UpdateOne(
		ctx,
		bson.M{"eventid": eventID},
		bson.M{"$set": updateFields},
	)
	if err != nil {
		return 0, err
	}
	return result.MatchedCount, nil
}

// func (repo *EventRepository) FindEventByID(ctx context.Context, eventID string) (structs.Event, error) {
// 	var event structs.Event
// 	err := repo.Collection.FindOne(ctx, bson.M{"eventid": eventID}).Decode(&event)
// 	return event, err
// }

func (repo *EventRepository) FindEvents(ctx context.Context, skip, limit int64, sort bson.D) ([]structs.Event, error) {
	cursor, err := repo.Collection.Find(ctx, bson.M{}, &options.FindOptions{
		Skip:  &skip,
		Limit: &limit,
		Sort:  sort,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []structs.Event
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}
