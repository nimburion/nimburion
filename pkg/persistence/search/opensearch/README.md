# `pkg/persistence/search/opensearch`

OpenSearch and Elasticsearch adapter for the search persistence family.

Owns:
- HTTP and optional SDK-backed connectivity for OpenSearch/Elasticsearch
- connection pooling, auth, health checks, and lifecycle
- indexing, deletion, and raw search execution helpers

Composition and wiring:
- construct `Adapter` directly where search infrastructure is required
- keep domain-specific search/query semantics above this package

Non-goals:
- generic store factories
- pretending search backends belong in a shared `store` umbrella

Testing:
- fast tests cover adapter behavior and SDK selection helpers

Status: target-state adapter package.
