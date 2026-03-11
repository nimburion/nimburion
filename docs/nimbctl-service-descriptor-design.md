# nimbctl Service Descriptor Design

This document defines the machine-readable service descriptor that `nimbctl` should consume from applications built with the new Nimburion framework pattern.

It complements:

- [architecture.md](./architecture.md)
- [archive/nimbctl-alignment-plan.md](./archive/nimbctl-alignment-plan.md)

## Purpose

The descriptor exists so `nimbctl` can interact with a Nimburion application without depending on:

- source tree heuristics
- AST parsing
- package layout assumptions
- JSON Schema as the primary integration contract

The descriptor is the primary contract for:

- application identity and kind discovery
- version compatibility policy between the app, Nimburion, the descriptor, and `nimbctl`
- default runtime invocation
- command discovery
- config render and validate capability discovery
- transport-family discovery, including gRPC-only applications
- runtime dependency declarations
- management endpoint and health semantics
- deployment capabilities
- migration policy metadata
- tenant and environment profile semantics
- feature maturity and stability metadata
- optional published artifacts such as JSON Schema

## Non-Goals

- It is not the application config itself.
- It is not a replacement for runtime health, metrics, or debug payloads.
- It is not a generic metadata dump for every possible feature detail.
- It is not the primary config-generation mechanism.

## Delivery Model

The descriptor must be available through a standard CLI command:

```bash
myapp describe --format json
```

Rules:

- output goes to `stdout`
- the command must not require a config file
- the command must not require external dependencies to be reachable
- the output must be deterministic for the same build

Optional secondary delivery:

- `nimburion.app.json`
- `nimburion.app.yaml`

If both CLI and file artifact exist, `nimbctl` should prefer the CLI command for local source trees and use the artifact only when CLI execution is not available or not desired.

## Versioning And Compatibility

The descriptor has its own version and it is independent from:

- application version
- framework version
- config schema version

Initial version:

- `descriptor_version: v1`

Compatibility rules:

- additive fields are allowed inside the same descriptor version
- renaming or removing required fields requires a new descriptor version
- `nimbctl` must evaluate descriptor compatibility before trying orchestration actions

The compatibility model for `v1` spans:

- `descriptor_version`
- `application.version`
- `compatibility.framework.version`
- `compatibility.nimbctl.supported_range`

## v1 Shape

The v1 descriptor is a JSON object with this top-level shape:

```json
{
  "descriptor_version": "v1",
  "application": {},
  "compatibility": {},
  "runtime": {},
  "commands": [],
  "config": {},
  "dependencies": [],
  "management": {},
  "transports": [],
  "deployment": {},
  "migrations": {},
  "features": [],
  "artifacts": {}
}
```

## Field Definitions

### `descriptor_version`

Required.

Type:

```json
"v1"
```

### `application`

Required.

Purpose:

- identify what the application is
- expose stable metadata that `nimbctl` needs for orchestration and workspace generation

Fields:

- `name`: required, string
- `kind`: required, enum
- `module`: optional, string
- `version`: optional, string
- `tenancy_mode`: optional, enum

Allowed `kind` values in `v1`:

- `gateway`
- `service`
- `consumer`
- `producer`
- `worker`
- `scheduler`

Allowed `tenancy_mode` values in `v1`:

- `single_tenant`
- `multi_tenant`
- `tenant_optional`

Example:

```json
"application": {
  "name": "api-gateway",
  "kind": "gateway",
  "module": "github.com/nimburion/apigateway",
  "version": "1.4.0",
  "tenancy_mode": "multi_tenant"
}
```

### `compatibility`

Required.

Purpose:

- make compatibility policy explicit for enterprise orchestration
- let `nimbctl` fail early when the service, framework, or descriptor contract is outside the supported range

Fields:

- `framework`: required, object
- `nimbctl`: required, object
- `policy`: optional, enum

#### `compatibility.framework`

Fields:

- `name`: required, string
- `version`: required, string

#### `compatibility.nimbctl`

Fields:

- `supported_range`: required, string
- `tested_range`: optional, string

Allowed `policy` values in `v1`:

- `strict`
- `best_effort`

Example:

```json
"compatibility": {
  "framework": {
    "name": "nimburion",
    "version": "0.18.0"
  },
  "nimbctl": {
    "supported_range": ">=1.0.0 <2.0.0",
    "tested_range": ">=1.0.0 <1.2.0"
  },
  "policy": "strict"
}
```

### `runtime`

Required.

Purpose:

- tell `nimbctl` how the application should be launched by default
- declare the standard input contract for runtime config files

Fields:

- `default_command`: required, string
- `config_file_flag`: optional, string
- `secrets_file_flag`: optional, string
- `supports_graceful_shutdown`: optional, boolean

Rules:

- `default_command` must reference a command present in `commands[].name`
- for Nimburion-generated applications, `default_command` should be `run`

Example:

```json
"runtime": {
  "default_command": "run",
  "config_file_flag": "--config-file",
  "secrets_file_flag": "--secret-file",
  "supports_graceful_shutdown": true
}
```

### `commands`

Required.

Purpose:

- tell `nimbctl` what commands the application exposes
- remove the need for AST parsing or command-policy heuristics

Each command entry has:

- `name`: required, string
- `path`: required, array of strings
- `kind`: required, enum
- `run_policy`: required, enum
- `default`: optional, boolean
- `transport`: optional, string

Allowed `kind` values in `v1`:

- `runtime`
- `transport`
- `config`
- `migration`
- `jobs`
- `scheduler`
- `maintenance`
- `debug`

Recommended `transport` values in `v1`:

- `http`
- `grpc`

Allowed `run_policy` values in `v1`:

- `always`
- `run`
- `migration`
- `scheduled`
- `manual`
- `on_demand`

Rules:

- `path` is the exact Cobra-style command path that `nimbctl` should invoke
- one command should have `default: true`, and it should match `runtime.default_command`
- `serve` is optional and should be represented as an HTTP transport command, not as the universal default
- gRPC transport commands may be represented with `transport: "grpc"` without implying that HTTP is present

### `config`

Required.

Purpose:

- tell `nimbctl` whether the application can render and validate its own config
- model regular and sensitive config inputs explicitly
- expose environment-profile semantics without forcing `nimbctl` to reverse-engineer them

Fields:

- `render`: required, object
- `validate`: required, object
- `inputs`: required, object
- `profiles`: optional, object
- `sensitivity_model`: optional, object

#### `config.render`

Fields:

- `supported`: required, boolean
- `path`: required when supported, array of strings
- `formats`: optional, array of strings
- `profiles`: optional, array of strings
- `outputs`: optional, array of output descriptors

Each output descriptor can contain:

- `name`: required, string
- `kind`: required, enum: `config` or `secrets`
- `format`: required, string
- `required`: optional, boolean

#### `config.validate`

Fields:

- `supported`: required, boolean
- `path`: required when supported, array of strings

#### `config.inputs`

Fields:

- `regular`: required, array of input descriptors
- `sensitive`: required, array of input descriptors

Each input descriptor can contain:

- `kind`: required, enum
- `flag`: optional, string
- `format`: optional, string
- `required`: optional, boolean
- `prefix`: optional, string
- `provider`: optional, string

Allowed `kind` values in `v1`:

- `config_file`
- `secrets_file`
- `env`
- `flag`
- `secret_provider`

Interpretation:

- `regular` declares non-sensitive config input channels
- `sensitive` declares how secrets or redacted values may enter the application
- `nimbctl` should use these fields to avoid guessing whether a value belongs in config, secrets, env, or an external secret source
- this is input-channel metadata, not field-value export

#### `config.sensitivity_model`

Optional.

Purpose:

- describe how the application models sensitive fields without exporting secret values to `nimbctl`

Fields:

- `classification_enum`: optional, array of strings
- `redaction_enum`: optional, array of strings
- `provenance_visible`: optional, boolean

Recommended `v1` values:

- `classification_enum`: `public`, `sensitive`, `secret`
- `redaction_enum`: `none`, `mask`, `full`

Interpretation:

- `nimbctl` can understand the service sensitivity policy at a metadata level
- field-level secret values remain owned by the service CLI and runtime
- provenance, if exposed, must be sanitized

#### `config.profiles`

Fields:

- `environments`: optional, array of profile descriptors

Each profile descriptor can contain:

- `name`: required, string
- `default`: optional, boolean
- `description`: optional, string

### `dependencies`

Optional.

Purpose:

- declare runtime dependencies in a way that is useful for enterprise orchestration
- let `nimbctl` distinguish hard dependencies from optional ones

Each dependency entry has:

- `name`: required, string
- `family`: required, string
- `role`: required, string
- `required`: required, boolean
- `readiness`: required, enum
- `durability`: optional, enum

Allowed `readiness` values in `v1`:

- `hard`
- `optional`

Allowed `durability` values in `v1`:

- `durable`
- `ephemeral`

Example:

```json
"dependencies": [
  {
    "name": "primary-db",
    "family": "persistence.relational",
    "role": "primary_store",
    "required": true,
    "readiness": "hard",
    "durability": "durable"
  },
  {
    "name": "cache",
    "family": "cache",
    "role": "response_cache",
    "required": false,
    "readiness": "optional",
    "durability": "ephemeral"
  }
]
```

### `management`

Optional.

Purpose:

- describe management-surface capabilities without embedding runtime health payloads into the descriptor
- let `nimbctl` understand endpoint semantics for health, readiness, liveness, metrics, or introspection

Fields:

- `supported`: required, boolean when present
- `transport`: optional, string
- `endpoints`: optional, array of endpoint descriptors

Each endpoint descriptor has:

- `kind`: required, enum
- `transport`: optional, string
- `target`: required, string
- `authenticated`: optional, boolean
- `reports`: optional, array of strings

Allowed `kind` values in `v1`:

- `health`
- `readiness`
- `liveness`
- `metrics`
- `introspection`

Recommended `transport` values in `v1`:

- `http`
- `grpc`

Recommended `reports` values:

- `hard_dependencies`
- `optional_dependencies`
- `degraded`
- `process_liveness`
- `feature_health`

Interpretation:

- `target` is transport-shaped, not always an HTTP path
- for HTTP management it is typically a path such as `/readyz`
- for gRPC management it is typically a fully-qualified service or service-method identifier such as `grpc.health.v1.Health/Check`
- `management.transport` provides the default transport for all endpoints in the block
- `endpoints[].transport`, when present, overrides the block default so one descriptor can model mixed HTTP and gRPC management surfaces

### `transports`

Optional.

Purpose:

- declare included transport families explicitly
- expose transport-family metadata without forcing `nimbctl` to infer it from commands alone
- keep gRPC runtime capabilities visible even when HTTP is absent

Each transport descriptor has:

- `family`: required, enum
- `services`: optional, array of service descriptors
- `reflection`: optional, object
- `health_service`: optional, object
- `security`: optional, object
- `proto_packages`: optional, array of proto package descriptors

Allowed `family` values in `v1`:

- `http`
- `grpc`

Each service descriptor has:

- `name`: required, string
- `package`: optional, string

`reflection` fields:

- `supported`: required, boolean when present

`health_service` fields:

- `supported`: required, boolean when present

`security` fields:

- `mode`: required when present, enum

Allowed `security.mode` values in `v1`:

- `insecure`
- `tls`
- `mtls`
- `external_termination`

Each proto package descriptor has:

- `name`: required, string
- `ownership`: optional, enum

Allowed `ownership` values in `v1`:

- `owned`
- `shared`
- `external`

### `deployment`

Optional.

Purpose:

- declare orchestration-relevant runtime capabilities
- keep deployment assumptions explicit instead of inferred from app kind alone

Fields:

- `mode`: required when present, enum
- `horizontal_scaling`: required when present, enum
- `capabilities`: optional, array of strings

Allowed `mode` values in `v1`:

- `stateless`
- `stateful`

Allowed `horizontal_scaling` values in `v1`:

- `safe`
- `leader_lock_required`
- `single_instance`

Recommended capability values:

- `requires_durable_store`
- `requires_durable_eventbus`
- `requires_leader_lock`
- `management_http`
- `management_grpc`
- `grpc_reflection`
- `grpc_public_transport`
- `mtls_required`
- `tenant_aware`

### `migrations`

Optional.

Purpose:

- make migration orchestration policy explicit
- let `nimbctl` know whether runtime startup is blocked by migration policy

Fields:

- `supported`: required, boolean when present
- `command`: required when supported, array of strings
- `strategy`: required when supported, enum
- `blocks_runtime_start`: optional, boolean
- `mixed_version_safe`: optional, boolean
- `requires_lock`: optional, boolean

Allowed `strategy` values in `v1`:

- `manual`
- `predeploy`
- `postdeploy`
- `online`
- `offline`
- `expand_contract`

### `features`

Required.

Purpose:

- classify the enabled application capabilities at a high level
- expose maturity and stability metadata useful for enterprise tooling

Type:

- array of feature descriptors

Each feature descriptor has:

- `name`: required, string
- `stability`: required, enum
- `criticality`: optional, enum

Allowed `stability` values in `v1`:

- `experimental`
- `beta`
- `stable`
- `deprecated`

Allowed `criticality` values in `v1`:

- `core`
- `optional`

Common feature names in `v1` may include:

- `http`
- `openapi`
- `auth`
- `grpc`
- `grpc_reflection`
- `grpc_health`
- `eventbus`
- `jobs`
- `scheduler`

Interpretation:

- `features` describes enabled capabilities and their maturity
- `transports` remains the canonical transport-family registry
- a gRPC application may expose `grpc_reflection` or `grpc_health` as separate optional capabilities without making them universal runtime assumptions

### `artifacts`

Optional.

Purpose:

- expose secondary published artifacts that external tools may use
- keep them outside the critical control path for `nimbctl`

Recommended fields:

- `config_schema`: optional object
- `proto_bundle`: optional object
- `grpc_descriptor_set`: optional object
- `grpc_buf_image`: optional object
- `grpc_contract_manifest`: optional object

`config_schema` fields:

- `format`: required, string
- `location`: required, string
- `version`: required, string

`proto_bundle`, `grpc_descriptor_set`, `grpc_buf_image`, and `grpc_contract_manifest` fields:

- `format`: required, string
- `location`: required, string
- `version`: optional, string

## Minimal v1 Example

```json
{
  "descriptor_version": "v1",
  "application": {
    "name": "orders-consumer",
    "kind": "consumer",
    "module": "github.com/example/orders-consumer",
    "version": "1.2.0",
    "tenancy_mode": "single_tenant"
  },
  "compatibility": {
    "framework": {
      "name": "nimburion",
      "version": "0.18.0"
    },
    "nimbctl": {
      "supported_range": ">=1.0.0 <2.0.0"
    },
    "policy": "strict"
  },
  "runtime": {
    "default_command": "run",
    "config_file_flag": "--config-file",
    "secrets_file_flag": "--secret-file",
    "supports_graceful_shutdown": true
  },
  "commands": [
    {
      "name": "run",
      "path": ["run"],
      "kind": "runtime",
      "run_policy": "run",
      "default": true
    },
    {
      "name": "config-validate",
      "path": ["config", "validate"],
      "kind": "config",
      "run_policy": "manual"
    }
  ],
  "config": {
    "render": {
      "supported": true,
      "path": ["config", "render"],
      "formats": ["yaml"],
      "profiles": ["minimal"],
      "outputs": [
        {
          "name": "config",
          "kind": "config",
          "format": "yaml",
          "required": true
        }
      ]
    },
    "validate": {
      "supported": true,
      "path": ["config", "validate"]
    },
    "inputs": {
      "regular": [
        {
          "kind": "config_file",
          "flag": "--config-file",
          "format": "yaml"
        }
      ],
      "sensitive": [
        {
          "kind": "secrets_file",
          "flag": "--secret-file",
          "format": "yaml"
        }
      ]
    },
    "sensitivity_model": {
      "classification_enum": ["public", "sensitive", "secret"],
      "redaction_enum": ["none", "mask", "full"],
      "provenance_visible": true
    },
    "profiles": {
      "environments": [
        {
          "name": "local",
          "default": true
        },
        {
          "name": "prod"
        }
      ]
    }
  },
  "dependencies": [
    {
      "name": "orders-eventbus",
      "family": "eventbus",
      "role": "consumer_transport",
      "required": true,
      "readiness": "hard",
      "durability": "durable"
    }
  ],
  "management": {
    "supported": false
  },
  "transports": [],
  "deployment": {
    "mode": "stateless",
    "horizontal_scaling": "safe"
  },
  "migrations": {
    "supported": false
  },
  "features": [
    {
      "name": "eventbus",
      "stability": "stable",
      "criticality": "core"
    }
  ],
  "artifacts": {}
}
```

## Gateway Example

```json
{
  "descriptor_version": "v1",
  "application": {
    "name": "api-gateway",
    "kind": "gateway",
    "module": "github.com/nimburion/apigateway",
    "version": "1.4.0",
    "tenancy_mode": "multi_tenant"
  },
  "compatibility": {
    "framework": {
      "name": "nimburion",
      "version": "0.18.0"
    },
    "nimbctl": {
      "supported_range": ">=1.0.0 <2.0.0",
      "tested_range": ">=1.0.0 <1.2.0"
    },
    "policy": "strict"
  },
  "runtime": {
    "default_command": "run",
    "config_file_flag": "--config-file",
    "secrets_file_flag": "--secret-file",
    "supports_graceful_shutdown": true
  },
  "commands": [
    {
      "name": "run",
      "path": ["run"],
      "kind": "runtime",
      "run_policy": "run",
      "default": true
    },
    {
      "name": "serve",
      "path": ["serve"],
      "kind": "transport",
      "run_policy": "run",
      "transport": "http"
    },
    {
      "name": "migrate",
      "path": ["migrate"],
      "kind": "migration",
      "run_policy": "migration"
    },
    {
      "name": "config-validate",
      "path": ["config", "validate"],
      "kind": "config",
      "run_policy": "manual"
    }
  ],
  "config": {
    "render": {
      "supported": true,
      "path": ["config", "render"],
      "formats": ["yaml"],
      "profiles": ["minimal", "full"],
      "outputs": [
        {
          "name": "config",
          "kind": "config",
          "format": "yaml",
          "required": true
        },
        {
          "name": "secrets",
          "kind": "secrets",
          "format": "yaml",
          "required": false
        }
      ]
    },
    "validate": {
      "supported": true,
      "path": ["config", "validate"]
    },
    "inputs": {
      "regular": [
        {
          "kind": "config_file",
          "flag": "--config-file",
          "format": "yaml"
        },
        {
          "kind": "env",
          "prefix": "APP"
        }
      ],
      "sensitive": [
        {
          "kind": "secrets_file",
          "flag": "--secret-file",
          "format": "yaml"
        },
        {
          "kind": "secret_provider",
          "provider": "aws-secrets-manager"
        }
      ]
    },
    "sensitivity_model": {
      "classification_enum": ["public", "sensitive", "secret"],
      "redaction_enum": ["none", "mask", "full"],
      "provenance_visible": true
    },
    "profiles": {
      "environments": [
        {
          "name": "dev"
        },
        {
          "name": "stage"
        },
        {
          "name": "prod",
          "default": true
        }
      ]
    }
  },
  "dependencies": [
    {
      "name": "primary-db",
      "family": "persistence.relational",
      "role": "primary_store",
      "required": true,
      "readiness": "hard",
      "durability": "durable"
    },
    {
      "name": "cache",
      "family": "cache",
      "role": "response_cache",
      "required": false,
      "readiness": "optional",
      "durability": "ephemeral"
    }
  ],
  "management": {
    "supported": true,
    "transport": "http",
    "endpoints": [
      {
        "kind": "liveness",
        "target": "/livez",
        "reports": ["process_liveness"]
      },
      {
        "kind": "readiness",
        "target": "/readyz",
        "reports": ["hard_dependencies", "optional_dependencies", "degraded"]
      },
      {
        "kind": "health",
        "target": "/healthz",
        "reports": ["feature_health", "hard_dependencies", "optional_dependencies"]
      }
    ]
  },
  "transports": [
    {
      "family": "http"
    }
  ],
  "deployment": {
    "mode": "stateless",
    "horizontal_scaling": "safe",
    "capabilities": ["requires_durable_store", "management_http", "tenant_aware"]
  },
  "migrations": {
    "supported": true,
    "command": ["migrate"],
    "strategy": "predeploy",
    "blocks_runtime_start": true,
    "mixed_version_safe": false,
    "requires_lock": true
  },
  "features": [
    {
      "name": "http",
      "stability": "stable",
      "criticality": "core"
    },
    {
      "name": "openapi",
      "stability": "stable",
      "criticality": "optional"
    },
    {
      "name": "auth",
      "stability": "beta",
      "criticality": "core"
    }
  ],
  "artifacts": {
    "config_schema": {
      "format": "jsonschema",
      "location": "config/schema.json",
      "version": "v1.0.0"
    }
  }
}
```

## gRPC Service Example

```json
{
  "descriptor_version": "v1",
  "application": {
    "name": "accounts-rpc",
    "kind": "service",
    "module": "github.com/example/accounts-rpc",
    "version": "1.0.0",
    "tenancy_mode": "tenant_optional"
  },
  "compatibility": {
    "framework": {
      "name": "nimburion",
      "version": "0.18.0"
    },
    "nimbctl": {
      "supported_range": ">=1.0.0 <2.0.0"
    },
    "policy": "strict"
  },
  "runtime": {
    "default_command": "run",
    "config_file_flag": "--config-file",
    "secrets_file_flag": "--secret-file",
    "supports_graceful_shutdown": true
  },
  "commands": [
    {
      "name": "run",
      "path": ["run"],
      "kind": "runtime",
      "run_policy": "run",
      "default": true
    },
    {
      "name": "serve-grpc",
      "path": ["serve", "grpc"],
      "kind": "transport",
      "run_policy": "run",
      "transport": "grpc"
    }
  ],
  "config": {
    "render": {
      "supported": false
    },
    "validate": {
      "supported": true,
      "path": ["config", "validate"]
    },
    "inputs": {
      "regular": [
        {
          "kind": "config_file",
          "flag": "--config-file",
          "format": "yaml"
        }
      ],
      "sensitive": [
        {
          "kind": "secrets_file",
          "flag": "--secret-file",
          "format": "yaml"
        }
      ]
    }
  },
  "dependencies": [
    {
      "name": "primary-db",
      "family": "persistence.relational",
      "role": "primary_store",
      "required": true,
      "readiness": "hard",
      "durability": "durable"
    }
  ],
  "management": {
    "supported": true,
    "transport": "grpc",
    "endpoints": [
      {
        "kind": "health",
        "target": "grpc.health.v1.Health/Check",
        "authenticated": false,
        "reports": ["hard_dependencies", "optional_dependencies", "degraded"]
      }
    ]
  },
  "transports": [
    {
      "family": "grpc",
      "services": [
        {
          "name": "AccountsService",
          "package": "accounts.v1"
        }
      ],
      "reflection": {
        "supported": true
      },
      "health_service": {
        "supported": true
      },
      "security": {
        "mode": "mtls"
      },
      "proto_packages": [
        {
          "name": "accounts.v1",
          "ownership": "owned"
        }
      ]
    }
  ],
  "deployment": {
    "mode": "stateless",
    "horizontal_scaling": "safe",
    "capabilities": ["management_grpc", "grpc_reflection", "grpc_public_transport", "mtls_required"]
  },
  "migrations": {
    "supported": false
  },
  "features": [
    {
      "name": "grpc",
      "stability": "beta",
      "criticality": "core"
    },
    {
      "name": "grpc_reflection",
      "stability": "beta",
      "criticality": "optional"
    },
    {
      "name": "grpc_health",
      "stability": "stable",
      "criticality": "core"
    }
  ],
  "artifacts": {
    "proto_bundle": {
      "format": "proto",
      "location": "api/proto"
    },
    "grpc_descriptor_set": {
      "format": "protobuf-descriptor-set",
      "location": "artifacts/accounts-rpc.pb"
    },
    "grpc_buf_image": {
      "format": "buf-image",
      "location": "artifacts/accounts-rpc.binpb"
    },
    "grpc_contract_manifest": {
      "format": "json",
      "location": "artifacts/grpc-contracts.json",
      "version": "v1"
    }
  }
}
```

## nimbctl Consumption Rules

`nimbctl` should use the descriptor in this order:

1. validate `descriptor_version`
2. validate `compatibility`
3. classify `application.kind`, `application.version`, and `application.tenancy_mode`
4. discover deployment constraints from `dependencies`, `deployment`, and `migrations`
5. identify the default runtime command from `runtime.default_command`
6. discover supported commands from `commands`
7. discover config render, validate, input, and environment profile semantics from `config`
8. discover management semantics from `management`
9. discover included transport families and gRPC-specific runtime metadata from `transports`
10. inspect feature maturity from `features`
11. inspect optional secondary artifacts from `artifacts`

`nimbctl` should not:

- infer commands from source tree layout
- infer command policies from AST parsing
- assume `serve` is the default command
- assume JSON Schema exists
- assume JSON Schema is needed for config generation
- guess which dependencies are hard or optional
- guess whether a service is stateless, leader-lock-bound, or migration-gated

## Why This Is Better Than JSON Schema As The Primary Contract

JSON Schema is good for:

- describing document shape
- editor autocomplete and validation
- external documentation tooling

It is weak as the primary `nimbctl` contract because it does not naturally express:

- executable commands
- default runtime behavior
- compatibility policy
- runtime dependency declarations
- management endpoint semantics
- transport-family metadata such as gRPC service exposure, reflection support, and transport security mode
- deployment capabilities
- migration policy
- sensitive config input handling
- tenant and environment profile semantics
- feature stability metadata

The descriptor fixes that by publishing orchestration metadata directly, while leaving schema documents optional.

## Open Decisions

These are still open, but they do not block the descriptor shape:

- final command name for config generation:
  - `config render`
  - `config init`
  - equivalent standardized name
- whether management endpoints should expose more standardized `reports` values
- whether remote catalogs should embed descriptor summaries directly or only point to descriptor sources

## Acceptance Criteria

The descriptor design is successful when:

- `nimbctl` can reject incompatible descriptors before trying orchestration actions
- `nimbctl` can classify runtime dependencies as hard or optional without probing application code
- `nimbctl` can understand management endpoint semantics without scraping route definitions
- `nimbctl` can understand management targets carried over HTTP paths or gRPC service or method identifiers without assuming HTTP-only management
- `nimbctl` can infer deployment constraints such as statelessness, leader-lock requirements, durable-store requirements, and migration policy from descriptor metadata
- `nimbctl` can distinguish config inputs from sensitive inputs and preserve that distinction during config generation flows
- `nimbctl` can read the service sensitivity model and sanitized provenance policy without receiving raw secret values
- `nimbctl` can read tenant mode, environment profiles, and feature stability directly from the descriptor
- `nimbctl` can discover gRPC-only applications and read their service, reflection, health-service, transport-security, and optional gRPC artifact metadata without requiring HTTP metadata
- JSON Schema remains optional and secondary
