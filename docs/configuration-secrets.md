# Configuration with Secrets Management

Nimburion supports separating sensitive configuration values into a dedicated secrets file for better security and GitOps workflows.

## How It Works

### Precedence Order
```
ENV variables > secrets file > config file > defaults
```

### File Discovery

The secrets file is automatically discovered using these rules:

1. **Environment variable**: `APP_SECRETS_FILE=/path/to/secrets.yaml`
2. **Same directory as config**: If you use `config.yaml`, it looks for `secrets.yaml`
3. **Current directory**: Looks for `secrets.yaml`, `secrets.yml`, `secrets.json`, or `secrets.toml`

### Example Setup

**config.yaml** (committed to Git):
```yaml
database:
  type: postgres
  host: localhost
  port: 5432
  database_name: myapp
  max_open_conns: 25

cache:
  type: redis
  max_conns: 10

auth:
  enabled: true
  issuer: https://auth.example.com
  audience: my-service
```

**secrets.yaml** (NOT committed to Git, in .gitignore):
```yaml
database:
  url: postgres://user:password@localhost:5432/myapp

cache:
  url: redis://:password@localhost:6379

auth:
  jwks_url: https://auth.example.com/.well-known/jwks.json
```

**Final merged configuration**:
```yaml
database:
  type: postgres
  host: localhost
  port: 5432
  database_name: myapp
  max_open_conns: 25
  url: postgres://user:password@localhost:5432/myapp  # from secrets

cache:
  type: redis
  max_conns: 10
  url: redis://:password@localhost:6379  # from secrets

auth:
  enabled: true
  issuer: https://auth.example.com
  audience: my-service
  jwks_url: https://auth.example.com/.well-known/jwks.json  # from secrets
```

## Usage

### In Code

```go
loader := config.NewViperLoader("config.yaml", "APP")

// Load with secrets support
cfg, secrets, err := loader.LoadWithSecrets()
if err != nil {
    log.Fatal(err)
}

// Show config with secrets redacted
fmt.Println(cfg.Redacted(secrets))

// Show full config (for debugging only)
fmt.Println(cfg.String())
```

### With CLI Helper

```bash
# Show config with secrets redacted (default)
myservice config show

# Show full config including secrets
myservice config show --show-secrets

# Validate configuration
myservice config validate
```

## Integration with Existing Code

### ViperLoader (Nimburion)

```go
// Old way (still works)
cfg, err := loader.Load()

// New way with secrets
cfg, secrets, err := loader.LoadWithSecrets()
```

### ConfigProvider (other applications)

The ConfigProvider continues to work as before. To add secrets support:

```go
provider := config.NewConfigProvider("config.yaml", "APP")

// Load secrets separately if needed
loader := config.NewViperLoader("config.yaml", "APP")
cfg, secrets, err := loader.LoadWithSecrets()
```

## Best Practices

### 1. Separate Concerns
- **config.yaml**: Non-sensitive settings (ports, timeouts, feature flags)
- **secrets.yaml**: Sensitive data (passwords, tokens, API keys)

### 2. GitOps Workflow
```bash
# .gitignore
secrets.yaml
secrets.yml
secrets.json
secrets.toml
```

### 3. Environment Variables Override
Even with secrets file, environment variables have highest precedence:
```bash
APP_DATABASE_URL="postgres://prod-user:prod-pass@prod-db:5432/prod" myservice serve
```

### 4. Kubernetes/Docker Secrets
Mount secrets as files:
```yaml
# kubernetes
volumes:
  - name: secrets
    secret:
      secretName: myapp-secrets
volumeMounts:
  - name: secrets
    mountPath: /etc/myapp/secrets.yaml
    subPath: secrets.yaml
```

Then set:
```bash
APP_SECRETS_FILE=/etc/myapp/secrets.yaml
```

## What Gets Redacted?

When you call `cfg.Redacted(secrets)`, any field that has a non-zero value in the secrets Config gets masked with `***`.

Example:
```go
// If secrets.yaml contains:
// database:
//   url: postgres://user:pass@localhost/db

// Then cfg.Redacted(secrets) shows:
// database:
//   url: ***
```

This ensures that when you log or display configuration, sensitive values are automatically hidden.
