# `pkg/persistence/object/s3`

S3-compatible adapter for the object persistence family.

Owns:
- object upload, download, delete, list, and presign behavior
- health checks and adapter lifecycle
- S3/AWS-specific configuration and transport details

Composition and wiring:
- construct `Adapter` directly where object storage is required
- keep higher-level file/document policies outside this package

Non-goals:
- generic store factories
- search, document, or SQL semantics

Testing:
- fast tests cover adapter behavior

Status: target-state adapter package.
