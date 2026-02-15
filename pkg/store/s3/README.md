# S3 Adapter

Adapter S3 (AWS SDK v2) per upload/download di documenti e immagini.

Funzioni principali:
- `Upload` / `UploadBytes`
- `Download`
- `Delete`
- `List`
- `PresignGetURL`
- `HealthCheck` / `Close`

## Config

```go
type Config struct {
  Bucket           string
  Region           string
  Endpoint         string
  AccessKeyID      string
  SecretAccessKey  string
  SessionToken     string
  UsePathStyle     bool
  OperationTimeout time.Duration
  PresignExpiry    time.Duration
}
```

## Esempio minimo

```go
adapter, err := s3.NewS3Adapter(s3.Config{
  Bucket: "documents",
  Region: "eu-west-1",
  // Endpoint: "http://localhost:4566", // LocalStack
  // AccessKeyID: "test",
  // SecretAccessKey: "test",
  UsePathStyle: true,
}, log)
if err != nil { panic(err) }
defer adapter.Close()

etag, err := adapter.UploadBytes(ctx, "invoices/2026-001.pdf", pdfBytes, "application/pdf", nil)
if err != nil { panic(err) }
_ = etag

url, err := adapter.PresignGetURL(ctx, "invoices/2026-001.pdf", 10*time.Minute)
if err != nil { panic(err) }
_ = url
```
