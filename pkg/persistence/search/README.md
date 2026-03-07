# `pkg/persistence/search`

Search persistence family root.

Owns backend-neutral placement for search-engine adapters and keeps search infrastructure separate from generic persistence umbrellas.

Composition and wiring:
- use backend adapters such as [`pkg/persistence/search/opensearch`](/Users/giuseppe/Workspace/github.com/nimburion/nimburion-refactoring-worktree/pkg/persistence/search/opensearch/README.md) directly
- keep domain-specific indexing and query semantics above this family root

Non-goals:
- generic legacy umbrella ownership
- SQL or object-storage responsibilities

Testing:
- backend packages own their behavior suites

Status: target-state family root.
