# SQS Adapter

Adapter EventBus per AWS SQS con publish, batch publish (chunk max 10), subscribe long-polling e health check.

Dipendenze:
- `github.com/aws/aws-sdk-go-v2/config`
- `github.com/aws/aws-sdk-go-v2/service/sqs`

## Config

```go
type Config struct {
  Region            string
  QueueURL          string
  Endpoint          string
  AccessKeyID       string
  SecretAccessKey   string
  SessionToken      string
  OperationTimeout  time.Duration
  WaitTimeSeconds   int32
  MaxMessages       int32
  VisibilityTimeout int32
}
```

Limitazioni:
- `PublishBatch` usa batch da massimo 10 messaggi (vincolo SQS)
- `Subscribe` lavora su code (topic come queue URL override)

## Esempio

```go
adapter, err := sqs.NewSQSAdapter(sqs.Config{
  Region: "eu-west-1",
  QueueURL: "http://localhost:4566/000000000000/my-queue",
  Endpoint: "http://localhost:4566", // LocalStack
  AccessKeyID: "test",
  SecretAccessKey: "test",
}, log)
if err != nil { panic(err) }
defer adapter.Close()
```
