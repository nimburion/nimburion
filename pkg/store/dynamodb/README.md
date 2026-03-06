# DynamoDB Adapter

> Transitional note: this README documents the current adapter under the legacy `pkg/store/*` layout. Keep it accurate for the current implementation, but treat `pkg/store` as transitional rather than the target module taxonomy for new code. See [docs/refactoring-requirements.md](../../../docs/refactoring-requirements.md).

Adapter DynamoDB (AWS SDK v2) con supporto endpoint custom (LocalStack), ping/health e helper CRUD principali.

Dipendenze:
- `github.com/aws/aws-sdk-go-v2/config`
- `github.com/aws/aws-sdk-go-v2/service/dynamodb`

## Config

```go
type Config struct {
  Region          string
  Endpoint        string
  AccessKeyID     string
  SecretAccessKey string
  SessionToken    string
  OperationTimeout time.Duration
}
```

## Esempio

```go
adapter, err := dynamodb.NewDynamoDBAdapter(dynamodb.Config{
  Region: "eu-west-1",
  Endpoint: "http://localhost:4566", // LocalStack
  AccessKeyID: "test",
  SecretAccessKey: "test",
}, log)
if err != nil { panic(err) }
```
