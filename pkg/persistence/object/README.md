# `pkg/persistence/object`

Object persistence family root.

Owns backend-neutral placement for object-storage adapters and keeps binary/blob storage separate from generic persistence umbrellas.

Composition and wiring:
- use backend adapters such as [`pkg/persistence/object/s3`](/Users/giuseppe/Workspace/github.com/nimburion/nimburion-refactoring-worktree/pkg/persistence/object/s3/README.md) directly
- keep file policies and application-level metadata above this family root

Non-goals:
- generic legacy umbrella ownership
- search, document, or SQL responsibilities

Testing:
- backend packages own their behavior suites

Status: target-state family root.
