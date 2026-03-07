# `pkg/persistence/keyvalue/dynamodb`

DynamoDB adapter for the key-value persistence family.

Owns:
- DynamoDB connectivity and lifecycle
- health checks and close semantics
- low-level item operations used by key-value and explicit document-style bridges

Composition and wiring:
- construct `Adapter` directly and inject it where DynamoDB is required
- document-style repository bridges live above this package in [`pkg/persistence/document`](/Users/giuseppe/Workspace/github.com/nimburion/nimburion-refactoring-worktree/pkg/persistence/document/README.md)

Non-goals:
- generic store factories
- pretending DynamoDB is the canonical document-family root

Testing:
- fast tests cover adapter behavior and property checks

Status: target-state adapter package.
