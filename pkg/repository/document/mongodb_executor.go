package document

import (
	"context"
	"fmt"

	mongostore "github.com/nimburion/nimburion/pkg/store/mongodb"
	"go.mongodb.org/mongo-driver/bson"
)

// MongoExecutor defines a minimal document execution contract for MongoDB-backed repositories.
type MongoExecutor interface {
	InsertOne(ctx context.Context, collection string, document map[string]interface{}) (interface{}, error)
	FindOne(ctx context.Context, collection string, filter Filter) (map[string]interface{}, error)
	UpdateOne(ctx context.Context, collection string, filter Filter, update map[string]interface{}) (int64, error)
	DeleteOne(ctx context.Context, collection string, filter Filter) (int64, error)
}

// MongoDBExecutor adapts store/mongodb adapter to the repository/document executor contract.
type MongoDBExecutor struct {
	adapter *mongostore.MongoDBAdapter
}

// NewMongoDBExecutor creates a new MongoDBExecutor instance.
func NewMongoDBExecutor(adapter *mongostore.MongoDBAdapter) (*MongoDBExecutor, error) {
	if adapter == nil {
		return nil, fmt.Errorf("mongodb adapter is required")
	}
	return &MongoDBExecutor{adapter: adapter}, nil
}

// InsertOne inserts a document into the collection.
func (e *MongoDBExecutor) InsertOne(ctx context.Context, collection string, document map[string]interface{}) (interface{}, error) {
	result, err := e.adapter.InsertOne(ctx, collection, bson.M(document))
	if err != nil {
		return nil, err
	}
	return result.InsertedID, nil
}

// FindOne finds a single document matching the filter.
func (e *MongoDBExecutor) FindOne(ctx context.Context, collection string, filter Filter) (map[string]interface{}, error) {
	out := bson.M{}
	if err := e.adapter.FindOne(ctx, collection, bson.M(filter), &out); err != nil {
		return nil, err
	}
	return map[string]interface{}(out), nil
}

// UpdateOne updates a single document matching the filter.
func (e *MongoDBExecutor) UpdateOne(ctx context.Context, collection string, filter Filter, update map[string]interface{}) (int64, error) {
	result, err := e.adapter.UpdateOne(ctx, collection, bson.M(filter), bson.M(update))
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

// DeleteOne deletes a single document matching the filter.
func (e *MongoDBExecutor) DeleteOne(ctx context.Context, collection string, filter Filter) (int64, error) {
	result, err := e.adapter.DeleteOne(ctx, collection, bson.M(filter))
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}
