# Document Repository Package

This package defines document-store repository contracts and backend executors.

## Why this package exists

`pkg/repository` is SQL-oriented (`SQLExecutor`, SQL query building).  
This package provides a dedicated home for document-family backends (MongoDB, DynamoDB)
without forcing SQL-shaped abstractions.

## Contracts

- `Reader[T, ID]`, `Writer[T, ID]`, `Repository[T, ID]`
- `Filter`, `Sort`, `Pagination`, `QueryOptions`

## Executors

- `MongoExecutor` / `MongoDBExecutor` backed by `pkg/store/mongodb`
- `DynamoExecutor` / `DynamoDBExecutor` backed by `pkg/store/dynamodb`

These executors are intentionally thin and backend-aware:
- Mongo uses flexible BSON filters/updates.
- Dynamo exposes expression-based APIs (key condition/update expressions).

