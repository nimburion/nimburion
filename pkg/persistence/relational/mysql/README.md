# `pkg/persistence/relational/mysql`

MySQL adapter for the relational persistence family.

Owns MySQL connectivity, connection pooling, health checks, and transaction-aware query execution.

## Config

```go
type Config struct {
  URL             string
  MaxOpenConns    int
  MaxIdleConns    int
  ConnMaxLifetime time.Duration
  QueryTimeout    time.Duration
}
```

Composition and wiring:
- construct `Adapter` directly and inject it into [`pkg/persistence/relational`](/Users/giuseppe/Workspace/github.com/nimburion/nimburion-refactoring-worktree/pkg/persistence/relational/README.md) repositories or service-layer SQL code
- keep dialect-portability concerns in the relational family, not in a generic store root

Dependency: `github.com/go-sql-driver/mysql`

## Esempio

```go
log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
adapter, err := mysql.NewMySQLAdapter(mysql.Config{
  URL: "user:pass@tcp(localhost:3306)/appdb?parseTime=true",
  MaxOpenConns: 25,
  MaxIdleConns: 5,
  ConnMaxLifetime: 5 * time.Minute,
  QueryTimeout: 10 * time.Second,
}, log)
if err != nil { panic(err) }
defer adapter.Close()
```

Non-goals:
- generic legacy umbrella ownership
- document or key-value semantics

Testing:
- fast tests cover adapter behavior and property checks

Status: target-state adapter package.
