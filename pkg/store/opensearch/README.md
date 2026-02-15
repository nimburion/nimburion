# OpenSearch/Elasticsearch Store Adapter

This package provides a shared store adapter for OpenSearch and Elasticsearch via HTTP APIs.
It also supports optional SDK-backed adapters (build-tag gated).

## Features

- Connection pooling with configurable limits
- Basic and API key authentication
- Health checks and connectivity verification
- Document indexing and deletion helpers
- Search query execution with raw JSON response

## Usage

```go
import (
    "context"
    "time"

    "github.com/nimburion/nimburion/pkg/observability/logger"
    "github.com/nimburion/nimburion/pkg/store/opensearch"
)

log, _ := logger.NewZapLogger(logger.Config{
    Level:  logger.InfoLevel,
    Format: logger.JSONFormat,
})

adapter, err := opensearch.NewOpenSearchAdapter(opensearch.Config{
    URL:              "http://localhost:9200",
    URLs:             []string{"http://localhost:9200", "http://localhost:9201"},
    APIKey:           "",
    Username:         "elastic",
    Password:         "changeme",
    AWSAuthEnabled:   false,
    AWSRegion:        "eu-west-1",
    AWSService:       "es",
    MaxConns:         10,
    OperationTimeout: 5 * time.Second,
}, log)
if err != nil {
    panic(err)
}
defer adapter.Close()

ctx := context.Background()
_ = adapter.IndexDocument(ctx, "products", "42", map[string]any{"name": "book"})
result, _ := adapter.Search(ctx, "products", map[string]any{
    "query": map[string]any{
        "match": map[string]any{"name": "book"},
    },
})
_ = result
```

## Configuration

| Field | Type | Description | Default |
| --- | --- | --- | --- |
| `URL` | `string` | OpenSearch/Elasticsearch base URL (required) | - |
| `URLs` | `[]string` | Cluster node URLs used for fallback/round-robin | `[]` |
| `Username` | `string` | Basic auth username (optional) | `""` |
| `Password` | `string` | Basic auth password (optional) | `""` |
| `APIKey` | `string` | API key auth token (optional, has precedence over basic auth) | `""` |
| `AWSAuthEnabled` | `bool` | Enable AWS SigV4 request signing | `false` |
| `AWSRegion` | `string` | AWS region (required with AWS auth) | `""` |
| `AWSService` | `string` | AWS service (`es` or `aoss`) | `es` |
| `AWSAccessKeyID` | `string` | Static AWS access key ID (optional) | `""` |
| `AWSSecretKey` | `string` | Static AWS secret key (optional) | `""` |
| `AWSSessionToken` | `string` | Static AWS session token (optional) | `""` |
| `MaxConns` | `int` | Max pooled HTTP connections | `10` |
| `OperationTimeout` | `time.Duration` | Request timeout | `5s` |

## Notes

- The adapter is compatible with both OpenSearch and Elasticsearch REST endpoints.
- Requests fallback across multiple configured nodes (`URLs`) on transport errors and `502/503/504`.
- Optional SDK drivers:
  - OpenSearch official client: build with `-tags opensearch_sdk`
  - Elasticsearch official client: build with `-tags elasticsearch_sdk`
- For domain-specific query/relevance logic, keep dedicated repositories in each bounded context and reuse this adapter as shared store infrastructure.
