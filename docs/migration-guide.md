# Migration Guide

This guide maps the pre-refactor Nimburion layout to the current target-state architecture on this branch.

## Core moves

- `pkg/server` -> `pkg/http/server` for HTTP bootstrap and `pkg/core/app` for runtime lifecycle
- `pkg/server/router` -> `pkg/http/router`
- `pkg/server/openapi` -> `pkg/http/openapi`
- `pkg/controller` -> `pkg/http/response`, `pkg/http/input`, and `pkg/core/errors`
- `pkg/configschema` -> `pkg/config/schema`

## Persistence and storage moves

- `pkg/repository` -> `pkg/persistence/relational`
- `pkg/repository/document` -> `pkg/persistence/document`
- `pkg/store/postgres` -> `pkg/persistence/relational/postgres`
- `pkg/store/mysql` -> `pkg/persistence/relational/mysql`
- `pkg/store/mongodb` -> `pkg/persistence/document/mongodb`
- `pkg/store/dynamodb` -> `pkg/persistence/keyvalue/dynamodb`
- `pkg/store/opensearch` -> `pkg/persistence/search/opensearch`
- `pkg/store/s3` -> `pkg/persistence/object/s3`
- `pkg/store/redis` and `pkg/store/memcached` -> role-specific packages under `pkg/cache/*`, `pkg/session/*`, and `pkg/coordination/*`

## HTTP family moves

- `pkg/middleware/*` -> `pkg/http/*` or `pkg/http/middleware/*`
- `pkg/realtime/sse` -> `pkg/http/sse`
- `pkg/realtime/ws` -> `pkg/http/ws`
- `pkg/middleware/openapivalidation` -> `pkg/http/contract/openapi`

## Shared family moves

- event durability and retry helpers moved out of `pkg/eventbus` into `pkg/reliability/*`
- authorization concepts moved out of HTTP-only ownership into `pkg/policy` and `pkg/tenant`
- runtime posture and flags live in `pkg/featureflag`
- file-path safety moved from `pkg/security` to `internal/safepath`

## CLI and descriptor model

- the root CLI is application-oriented and uses `run` as the canonical runtime command
- family commands such as `migrate`, `openapi`, `jobs`, `scheduler`, and `cache` are contributed by their owning families
- service metadata is published through `describe --format json` and modeled in `pkg/descriptor`

## Config model

- `pkg/config` is now the root aggregate and loader/provider orchestration package
- section ownership, defaults, env binding, validation, and schema contribution live in the relevant family packages
- app identity moved from `service.*` to `app.*`
- legacy env aliases such as `APP_MANAGEMENT_*` and `APP_DATABASE_*` are removed on this branch; use `APP_MGMT_*` and `APP_DB_*`

## What to update in downstream code

- replace imports of removed legacy roots with their target-state packages
- stop assuming `serve` is the default runtime command
- stop depending on central factory packages when the owning family now exposes explicit construction or feature contributions
- move any new config logic into family-owned `config` packages instead of adding it to `pkg/config`
