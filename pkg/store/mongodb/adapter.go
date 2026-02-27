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

// Adapter provides MongoDB connectivity.
type Adapter struct {
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
// Esempio minimo: adapter, err := mongodb.NewAdapter(cfg, log)
func NewAdapter(cfg Config, log logger.Logger) (*Adapter, error) {
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
	return &Adapter{
		client:   client,
		database: cfg.Database,
		logger:   log,
		timeout:  cfg.OperationTimeout,
	}, nil
}

func (a *Adapter) Client() *mongo.Client {
	return a.client
}

func (a *Adapter) Database() *mongo.Database {
	return a.client.Database(a.database)
}

func (a *Adapter) Collection(name string) *mongo.Collection {
	return a.Database().Collection(name)
}

func (a *Adapter) Ping(ctx context.Context) error {
	a.mu.RLock()
	closed := a.closed
	a.mu.RUnlock()
	if closed {
		return fmt.Errorf("mongodb adapter is closed")
	}
	return a.client.Ping(ctx, readpref.Primary())
}

func (a *Adapter) HealthCheck(ctx context.Context) error {
	hcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := a.Ping(hcCtx); err != nil {
		a.logger.Error("MongoDB health check failed", "error", err)
		return fmt.Errorf("mongodb health check failed: %w", err)
	}
	return nil
}

func (a *Adapter) Close() error {
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
func (a *Adapter) InsertOne(ctx context.Context, collection string, doc interface{}) (*mongo.InsertOneResult, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.Collection(collection).InsertOne(opCtx, doc)
}

func (a *Adapter) FindOne(ctx context.Context, collection string, filter interface{}, result interface{}) error {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.Collection(collection).FindOne(opCtx, filter).Decode(result)
}

func (a *Adapter) UpdateOne(ctx context.Context, collection string, filter, update interface{}) (*mongo.UpdateResult, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.Collection(collection).UpdateOne(opCtx, filter, update)
}

func (a *Adapter) DeleteOne(ctx context.Context, collection string, filter interface{}) (*mongo.DeleteResult, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.Collection(collection).DeleteOne(opCtx, filter)
}

func (a *Adapter) EnsureCollection(ctx context.Context, name string) error {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	_, err := a.Database().Collection(name).CountDocuments(opCtx, bson.D{})
	return err
}

func (a *Adapter) withOperationTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.timeout <= 0 {
		return ctx, func() {}
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, a.timeout)
}
