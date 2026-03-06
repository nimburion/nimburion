# SSE package

> Transitional note: this README documents the current `pkg/realtime/sse` packaging. The SSE runtime remains valid, but the target refactor treats realtime HTTP capabilities as part of the HTTP family rather than a stable standalone top-level package. See [docs/refactoring-requirements.md](../../../docs/refactoring-requirements.md).

`pkg/realtime/sse` provides a router-agnostic Server-Sent Events runtime with:

- local connection manager (`Manager`)
- subscribe/publish/disconnect APIs
- heartbeat and retry support
- Last-Event-ID short replay (`Store`)
- distributed fan-out (`Bus`)

## Components

- `handler.go`: HTTP SSE adapter using `router.Context`
- `manager.go`: local subscribers, routing (tenant/subject/channel), fan-out
- `store_inmemory.go`, `store_redis.go`: replay stores
- `bus.go`: in-memory and Redis pub/sub buses for multi-instance delivery
- `bus_eventbus.go`: adapter to framework `pkg/eventbus` (Kafka/RabbitMQ/SQS)

## Typical setup

```go
replayStore := sse.NewInMemoryStore(256) // or sse.NewRedisStore(...)
bus := sse.NewInMemoryBus() // or sse.NewRedisBus(...)

manager := sse.NewManager(sse.DefaultManagerConfig(), replayStore, bus)
handler, _ := sse.NewHandler(sse.HandlerConfig{Manager: manager})

r.GET("/events", handler.Stream())
```

Use existing middleware on `/events`:

- `authz.Authenticate(...)`
- `authz.ClaimsGuard(...)`
- `ratelimit.RateLimit(...)`
- `securityheaders.Middleware(...)`
- `metrics.Metrics()`
