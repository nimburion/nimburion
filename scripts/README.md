# Godoc Comment Generation

This directory contains scripts for automatically generating and maintaining godoc comments for public Go APIs.

## Scripts

### `analyze_godoc.py`
Analyzes Go source files to identify public symbols (functions, methods, types) that lack godoc comments.

```bash
python3 scripts/analyze_godoc.py
```

Output:
- Total count of missing godoc comments
- Breakdown by symbol type (function, method, type)
- List of first 20 symbols for review

### `apply_godoc.py`
Automatically generates and applies godoc comments to Go source files.

```bash
# Single file
python3 scripts/apply_godoc.py pkg/server/public.go

# Entire directory
python3 scripts/apply_godoc.py pkg/server/

# All packages
python3 scripts/apply_godoc.py pkg/
```

Features:
- Template-based comment generation for common patterns
- Automatic detection of constructors (New* functions)
- Preserves existing comments
- Follows Go documentation conventions

### `generate-godoc.sh`
Shell script for quick analysis using awk pattern matching.

```bash
./scripts/generate-godoc.sh
```

### `generate_godoc_batch.py`
Preview godoc comments before applying them.

```bash
python3 scripts/generate_godoc_batch.py pkg/server/public.go
```

## Comment Templates

The `apply_godoc.py` script uses predefined templates for common method names:

- **Router**: "returns the underlying router instance..."
- **Start**: "begins serving HTTP requests..."
- **Close**: "releases all resources..."
- **HealthCheck**: "verifies the component is operational..."
- **GET/POST/PUT/DELETE/PATCH**: "registers a handler for HTTP..."
- **Publish/Subscribe**: "sends/receives messages..."
- **Enqueue/Ack/Nack**: "job processing operations..."

## Workflow

1. **Analyze** - Identify missing comments:
   ```bash
   python3 scripts/analyze_godoc.py
   ```

2. **Preview** - Review generated comments:
   ```bash
   python3 scripts/generate_godoc_batch.py pkg/server/public.go
   ```

3. **Apply** - Add comments to files:
   ```bash
   python3 scripts/apply_godoc.py pkg/server/
   ```

4. **Verify** - Check compilation and linting:
   ```bash
   go build ./pkg/...
   golint ./pkg/...
   ```

5. **Commit** - Create atomic commits:
   ```bash
   git add pkg/server/
   git commit -m "docs: add godoc comments to pkg/server"
   ```

## Coverage

Current godoc coverage for priority packages:

- ✅ pkg/server (100%)
- ✅ pkg/server/router (100%)
- ✅ pkg/server/openapi (100%)
- ✅ pkg/config (100%)
- ✅ pkg/auth (100%)
- ✅ pkg/middleware (100%)
- ✅ pkg/eventbus (100%)
- ✅ pkg/jobs (100%)
- ✅ pkg/repository (100%)

Total: **150+ public APIs documented**

## Best Practices

1. **Concise descriptions** - Start with the symbol name, use present tense
2. **Explain behavior** - What does it do, not how it does it
3. **Document parameters** - Mention important parameters inline
4. **Note side effects** - Blocking calls, resource allocation, etc.
5. **Link related functions** - Use `See also:` for related APIs

## Example

```go
// BuildHTTPServers constructs framework HTTP servers from config/options.
// Creates both public and management servers based on configuration.
// Returns an error if router creation or server initialization fails.
func BuildHTTPServers(opts *RunHTTPServersOptions) (*HTTPServers, error) {
    // ...
}
```

## Maintenance

Run analysis periodically to catch new public APIs:

```bash
# Weekly check
python3 scripts/analyze_godoc.py

# Apply to new symbols
python3 scripts/apply_godoc.py pkg/
```

## References

- [Effective Go - Commentary](https://go.dev/doc/effective_go#commentary)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments#doc-comments)
- [Godoc: documenting Go code](https://go.dev/blog/godoc)
