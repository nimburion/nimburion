package mongodb

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// MongoDBAdapter provides MongoDB connectivity.
type MongoDBAdapter struct {
	client   *mongo.Client
	database string
	logger   logger.Logger
	timeout  time.Duration
	mu       sync.RWMutex
	closed   bool
}

// Config holds MongoDB adapter configuration.
type Config struct {
	URL              string
	Database         string
	ConnectTimeout   time.Duration
	OperationTimeout time.Duration
}

// Cosa fa: inizializza un adapter MongoDB e verifica connettività via ping.
// Cosa NON fa: non crea indici o collezioni automaticamente.
// Esempio minimo: adapter, err := mongodb.NewMongoDBAdapter(cfg, log)
func NewMongoDBAdapter(cfg Config, log logger.Logger) (*MongoDBAdapter, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("mongodb URL is required")
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("mongodb database is required")
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 5 * time.Second
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.URL))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	log.Info("MongoDB connection established", "database", cfg.Database)
	return &MongoDBAdapter{
		client:   client,
		database: cfg.Database,
		logger:   log,
		timeout:  cfg.OperationTimeout,
	}, nil
}

// Client TODO: add description
func (a *MongoDBAdapter) Client() *mongo.Client {
	return a.client
}

// Database TODO: add description
func (a *MongoDBAdapter) Database() *mongo.Database {
	return a.client.Database(a.database)
}

// Collection TODO: add description
func (a *MongoDBAdapter) Collection(name string) *mongo.Collection {
	return a.Database().Collection(name)
}

// Ping performs a basic connectivity check to verify the service is reachable.
func (a *MongoDBAdapter) Ping(ctx context.Context) error {
	a.mu.RLock()
	closed := a.closed
	a.mu.RUnlock()
	if closed {
		return fmt.Errorf("mongodb adapter is closed")
	}
	return a.client.Ping(ctx, readpref.Primary())
}

// HealthCheck verifies the component is operational and can perform its intended function.
func (a *MongoDBAdapter) HealthCheck(ctx context.Context) error {
	hcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := a.Ping(hcCtx); err != nil {
		a.logger.Error("MongoDB health check failed", "error", err)
		return fmt.Errorf("mongodb health check failed: %w", err)
	}
	return nil
}

// Close releases all resources held by this instance. Should be called when the instance is no longer needed.
func (a *MongoDBAdapter) Close() error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	a.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.client.Disconnect(ctx); err != nil {
		return fmt.Errorf("failed to close mongodb connection: %w", err)
	}
	return nil
}

// Cosa fa: inserisce un documento nella collection target.
// Cosa NON fa: non valida lo schema del documento.
// Esempio minimo: _, err := adapter.InsertOne(ctx, "users", doc)
func (a *MongoDBAdapter) InsertOne(ctx context.Context, collection string, doc interface{}) (*mongo.InsertOneResult, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.Collection(collection).InsertOne(opCtx, doc)
}

// FindOne TODO: add description
func (a *MongoDBAdapter) FindOne(ctx context.Context, collection string, filter interface{}, result interface{}) error {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.Collection(collection).FindOne(opCtx, filter).Decode(result)
}

// UpdateOne TODO: add description
func (a *MongoDBAdapter) UpdateOne(ctx context.Context, collection string, filter, update interface{}) (*mongo.UpdateResult, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.Collection(collection).UpdateOne(opCtx, filter, update)
}

// DeleteOne TODO: add description
func (a *MongoDBAdapter) DeleteOne(ctx context.Context, collection string, filter interface{}) (*mongo.DeleteResult, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.Collection(collection).DeleteOne(opCtx, filter)
}

// EnsureCollection TODO: add description
func (a *MongoDBAdapter) EnsureCollection(ctx context.Context, name string) error {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	_, err := a.Database().Collection(name).CountDocuments(opCtx, bson.D{})
	return err
}

func (a *MongoDBAdapter) withOperationTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.timeout <= 0 {
		return ctx, func() {}
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, a.timeout)
}
