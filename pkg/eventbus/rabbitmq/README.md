# RabbitMQ Adapter

Adapter EventBus RabbitMQ con supporto publish, batch publish, subscribe con ack/nack e health check.

Dipendenza: `github.com/rabbitmq/amqp091-go`

## Config

```go
type Config struct {
  URL              string
  Exchange         string
  ExchangeType     string
  QueueName        string
  RoutingKey       string
  OperationTimeout time.Duration
  ConsumerTag      string
}
```

## Esempio

```go
adapter, err := rabbitmq.NewRabbitMQAdapter(rabbitmq.Config{
  URL: "amqp://guest:guest@localhost:5672/",
  Exchange: "events",
  ExchangeType: "topic",
  RoutingKey: "orders.*",
}, log)
if err != nil { panic(err) }
defer adapter.Close()
```
