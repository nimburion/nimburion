# Comment Templates for Remaining Exports

Use these templates to quickly add comments to remaining exported types.

## Constants

```go
// ConstantName describes what this constant represents
const ConstantName = "value"
```

## Types

```go
// TypeName describes the purpose of this type
type TypeName struct {
    // ...
}
```

## Functions

```go
// FunctionName does something specific
func FunctionName() {}
```

## Methods

```go
// MethodName performs an action on the receiver
func (r *Receiver) MethodName() {}
```

## Quick Patterns

### For "should have comment or be unexported"
If the export is intentional, add a comment. If not, make it unexported (lowercase).

### For "comment should be of the form 'Name ...'"
Start the comment with the exact name:
```go
// NewThing creates a new Thing
func NewThing() *Thing {}
```

### For stuttering (e.g., `kafka.KafkaAdapter`)
Rename to avoid repetition:
```go
// Adapter implements Kafka event bus adapter
type Adapter struct {} // instead of KafkaAdapter
```

### For constants blocks
Add a group comment:
```go
// HTTP method constants
const (
    // MethodGet represents HTTP GET
    MethodGet = "GET"
    // MethodPost represents HTTP POST
    MethodPost = "POST"
)
```

## Priority Order

1. **Public API** (pkg/server, pkg/config, pkg/middleware)
2. **Core types** (pkg/eventbus, pkg/jobs, pkg/repository)
3. **Adapters** (pkg/store/*, pkg/email/*)
4. **Internal utilities** (everything else)

## Automation Helper

For simple constant blocks, use this sed pattern:
```bash
# Add comment above constant
sed -i '/^const ConstName/i\
// ConstName description
' file.go
```

For methods, manually review and add appropriate comments based on functionality.
