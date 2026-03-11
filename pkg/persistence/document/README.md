# `pkg/persistence/document`

Document-family persistence contracts and backend-aware executors.

Owns:
- `Reader[T, ID]`, `Writer[T, ID]`, `Repository[T, ID]`
- `Filter`, `Sort`, `Pagination`, `QueryOptions`
- executor contracts for MongoDB and DynamoDB bridges

Composition and wiring:
- use [`pkg/persistence/document/mongodb`](/Users/giuseppe/Workspace/github.com/nimburion/nimburion-refactoring-worktree/pkg/persistence/document/mongodb/README.md) for MongoDB adapters
- use [`pkg/persistence/keyvalue/dynamodb`](/Users/giuseppe/Workspace/github.com/nimburion/nimburion-refactoring-worktree/pkg/persistence/keyvalue/dynamodb/README.md) when DynamoDB is used explicitly as a document-style bridge
- keep repository contracts backend-aware where the family semantics differ materially from SQL

Non-goals:
- SQL portability
- generic legacy umbrella abstractions
- HTTP or transport ownership

Testing:
- fast tests cover contracts and executor validation
- adapter-specific behavior is tested in each backend package

Status: target-state family root.
