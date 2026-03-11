# `pkg/persistence/document/mongodb`

MongoDB adapter for the document persistence family.

Owns:
- MongoDB connectivity and lifecycle
- health checks and close semantics
- collection-level CRUD helpers used by document executors

Composition and wiring:
- construct `Adapter` directly and inject it into [`pkg/persistence/document`](/Users/giuseppe/Workspace/github.com/nimburion/nimburion-refactoring-worktree/pkg/persistence/document/README.md) executors
- keep MongoDB-specific BSON behavior in this package, not in family-agnostic contracts

Non-goals:
- generic store factories
- SQL-style repository abstractions

Testing:
- fast tests cover adapter behavior and property checks

Status: target-state adapter package.
