# MySQL Adapter

Adapter MySQL con pool connessioni, health check e supporto transazionale.

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

Dipendenza: `github.com/go-sql-driver/mysql`

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
