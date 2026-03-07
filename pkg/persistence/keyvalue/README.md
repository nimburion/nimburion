# `pkg/persistence/keyvalue`

Key-value persistence family root.

Owns backend-neutral positioning for key-value stores and the adapter packages that implement those semantics.

Composition and wiring:
- use backend adapters such as [`pkg/persistence/keyvalue/dynamodb`](/Users/giuseppe/Workspace/github.com/nimburion/nimburion-refactoring-worktree/pkg/persistence/keyvalue/dynamodb/README.md) directly
- keep document or search semantics out of this family root

Non-goals:
- generic legacy umbrella ownership
- SQL or document repository abstractions

Testing:
- backend packages own their behavior suites

Status: target-state family root.
