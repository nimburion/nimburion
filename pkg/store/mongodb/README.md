# MongoDB Adapter

> Transitional note: this README documents the current adapter under the legacy `pkg/store/*` layout. Keep it accurate for the current implementation, but treat `pkg/store` as transitional rather than the target module taxonomy for new code. See [docs/refactoring-requirements.md](../../../docs/refactoring-requirements.md).

Adapter MongoDB con ping, health check, close e helper CRUD base.

Dipendenza: `go.mongodb.org/mongo-driver`

## Config

```go
type Config struct {
  URL              string
  Database         string
  ConnectTimeout   time.Duration
  OperationTimeout time.Duration
}
```

## Esempio

```go
log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
adapter, err := mongodb.NewMongoDBAdapter(mongodb.Config{
  URL: "mongodb://localhost:27017",
  Database: "appdb",
  ConnectTimeout: 5 * time.Second,
}, log)
if err != nil { panic(err) }
defer adapter.Close()
```
