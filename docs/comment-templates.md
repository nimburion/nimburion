# Comment Guide for Exported API

Use this guide when writing comments for exported symbols in Nimburion during the refactor.

The goal is not to satisfy lint with low-value comments. The goal is to make public contracts understandable.

## Rule Of Thumb

An exported comment should answer at least one of these:

- what contract does this symbol expose?
- when should it be used?
- what responsibility does it own?
- what does it explicitly not do?

If the comment only restates the name, it is probably not useful.

## Prefer Contract Comments Over Filler

Weak:

```go
// Manager manages things.
type Manager struct {}
```

Better:

```go
// Manager coordinates local SSE subscribers and fan-out for one process.
type Manager struct {}
```

Weak:

```go
// Validate validates the config.
func Validate(cfg Config) error {}
```

Better:

```go
// Validate checks semantic configuration rules after file, env, and flag values are merged.
func Validate(cfg Config) error {}
```

## When Not To Export

If a symbol is not part of the intended public contract:

- make it unexported instead of documenting it
- avoid exporting helpers only because they are convenient in one package
- keep vendor plumbing and glue code in `internal/...` when it is not part of the framework API

## Comment Patterns By Symbol Kind

## Packages

Package comments should describe:

- the family or role the package represents
- the level of abstraction it exposes
- the kinds of implementations expected under it

Example:

```go
// Package cache defines the cache role contracts used by framework features and applications.
package cache
```

## Interfaces

Interface comments should explain:

- the contract boundary
- the minimum semantics callers can rely on
- portability limits when relevant

Template:

```go
// InterfaceName defines the minimum contract for ...
//
// Callers may rely on ...
// Implementations are expected to ...
type InterfaceName interface {
    // ...
}
```

Example:

```go
// EventBus defines the durable messaging contract used by publishers and consumers.
//
// Callers may rely on publish, subscribe, and health semantics only.
// Broker-specific routing and partition details are not part of this contract.
type EventBus interface {
    // ...
}
```

## Struct Types

Struct comments should say whether the type is:

- a config object
- a runtime component
- an adapter
- a request/response model
- an error or descriptor

Template:

```go
// TypeName describes ...
type TypeName struct {
    // ...
}
```

Examples:

```go
// Config holds the HTTP server settings owned by the HTTP feature.
type Config struct {
    // ...
}
```

```go
// Descriptor describes the machine-readable application contract consumed by nimbctl.
type Descriptor struct {
    // ...
}
```

## Config Structs

Config comments should state:

- which feature owns the config
- whether it is core or feature-local
- whether it is runtime config, descriptor config, or generation config

Example:

```go
// Config holds the settings for the relational migration feature.
type Config struct {
    // ...
}
```

## Constructors

Constructor comments should say what is built and at what level.

Avoid:

```go
// New creates a new X.
```

Prefer:

```go
// New builds a Redis-backed session store.
func New(cfg Config, log logger.Logger) (*Store, error) {}
```

If there are important constraints, say them:

```go
// New builds an HTTP server bound to the provided router and does not start listening.
func New(cfg Config, r router.Router) (*Server, error) {}
```

## Methods

Method comments should explain behavior, not repeat the method name.

Good cases for comments:

- non-obvious side effects
- lifecycle behavior
- idempotency expectations
- transport or concurrency semantics

Example:

```go
// Run starts the application lifecycle and blocks until shutdown or error.
func (a *App) Run(ctx context.Context) error {}
```

```go
// Register contributes HTTP routes for this feature without starting a server.
func (f *Feature) Register(r router.Router) {}
```

## Errors

Error type comments should explain when the error is returned and what it means to callers.

Example:

```go
// OptimisticLockError reports that an update failed because the stored version no longer matches the caller's version.
type OptimisticLockError struct {
    // ...
}
```

## Constants

Constant comments should explain the domain meaning, not only the literal.

Example:

```go
// PolicyMigration marks commands that are intended to run during migration workflows.
const PolicyMigration CommandPolicy = "migration"
```

For constant blocks, add a block comment only when the constants clearly belong to one domain:

```go
// Command execution policy values.
const (
    // PolicyAlways allows the command to run in every workflow.
    PolicyAlways CommandPolicy = "always"
    // PolicyManual marks commands intended for explicit operator use.
    PolicyManual CommandPolicy = "manual"
)
```

## Descriptor And Contract Types

These are especially important in the new refactor.

Comments should say:

- who consumes the contract
- whether the contract is stable
- whether fields are optional or versioned

Example:

```go
// Descriptor is the machine-readable application contract consumed by nimbctl.
//
// It is versioned independently from the framework and is intended for orchestration,
// command discovery, and config capability discovery.
type Descriptor struct {
    // ...
}
```

## Feature Types

Feature comments should explain what the feature contributes.

Example:

```go
// Feature contributes HTTP routes, config sections, and health checks for the OpenAPI capability.
type Feature struct {
    // ...
}
```

## Adapters And Providers

Adapter comments should say:

- what family or role they implement
- which vendor/backend they use
- what level of the contract they cover

Example:

```go
// Store implements the cache.Store contract on top of Redis.
type Store struct {
    // ...
}
```

```go
// Validator implements event contract validation with Protobuf descriptors.
type Validator struct {
    // ...
}
```

## Package Priority During Refactor

When adding or fixing comments, prioritize these areas first:

1. `pkg/core`
2. `pkg/cli`
3. `pkg/config` and `pkg/config/schema`
4. `pkg/http`
5. `pkg/persistence/*`
6. `pkg/eventbus`, `pkg/jobs`, `pkg/scheduler`, `pkg/coordination`, `pkg/pubsub`
7. `pkg/auth`, `pkg/email`, `pkg/health`, `pkg/observability`, `pkg/resilience`, `pkg/version`

These are the packages most likely to define the public contract of the refactored framework.

## Style Rules

- Start the comment with the exact exported name.
- Keep the first sentence short and contract-focused.
- Add a second sentence only when it clarifies behavior, ownership, or limits.
- Avoid "helper", "utility", "manager", "handler" unless the next words explain the real responsibility.
- Avoid comments like:
  - `X is X`
  - `NewX creates a new X`
  - `Handle handles the request`
  - `Config contains configuration`

## Quick Templates

Use these only as starting points.

### Interface

```go
// InterfaceName defines the minimum contract for ...
//
// Callers may rely on ...
type InterfaceName interface {
    // ...
}
```

### Config

```go
// Config holds the settings for ...
type Config struct {
    // ...
}
```

### Constructor

```go
// New builds a ... and does not ...
func New(...) (...) {}
```

### Runtime Method

```go
// Run starts ... and blocks until ...
func (x *TypeName) Run(ctx context.Context) error {}
```

### Error

```go
// ErrorName reports that ...
type ErrorName struct {
    // ...
}
```

## Final Check Before Writing A Comment

Before you add a comment, ask:

1. Should this symbol be exported at all?
2. Does the comment explain the contract or only restate the name?
3. Would a new contributor understand when to use this symbol after reading the comment?
