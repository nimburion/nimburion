// Package postgres provides a PostgreSQL-backed coordination lock provider.
package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	// Register the lib/pq SQL driver for sql.Open("postgres", ...).
	_ "github.com/lib/pq"

	"github.com/nimburion/nimburion/pkg/coordination"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

func coordinationError(kind error, message string) error {
	switch {
	case errors.Is(kind, coordination.ErrValidation):
		return coreerrors.New("validation.coordination", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(400)
	case errors.Is(kind, coordination.ErrInvalidArgument):
		return coreerrors.New("argument.coordination.invalid", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(400)
	case errors.Is(kind, coordination.ErrConflict):
		return coreerrors.New("coordination.conflict", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(409)
	case errors.Is(kind, coordination.ErrRetryable):
		return coreerrors.NewRetryable(messageOrDefault(message, kind.Error()), kind).
			WithDetails(map[string]interface{}{"family": "coordination"})
	case errors.Is(kind, coordination.ErrNotInitialized):
		return coreerrors.NewNotInitialized(messageOrDefault(message, kind.Error()), kind).
			WithDetails(map[string]interface{}{"family": "coordination"})
	default:
		if message == "" {
			return kind
		}
		return fmt.Errorf("%w: %s", kind, message)
	}
}

const (
	defaultPostgresLockTable     = "nimburion_coordination_locks"
	defaultPostgresLockOperation = 3 * time.Second
)

var validTableName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// PostgresLockProviderConfig configures Postgres lock provider.
type PostgresLockProviderConfig struct {
	URL              string
	Table            string
	OperationTimeout time.Duration
}

func (c *PostgresLockProviderConfig) normalize() {
	if strings.TrimSpace(c.Table) == "" {
		c.Table = defaultPostgresLockTable
	}
	if c.OperationTimeout <= 0 {
		c.OperationTimeout = defaultPostgresLockOperation
	}
}

// PostgresLockProvider stores distributed lock rows in Postgres.
type PostgresLockProvider struct {
	db     *sql.DB
	log    logger.Logger
	config PostgresLockProviderConfig
}

// NewPostgresLockProvider creates lock provider backed by Postgres table rows.
func NewPostgresLockProvider(cfg PostgresLockProviderConfig, log logger.Logger) (*PostgresLockProvider, error) {
	if log == nil {
		return nil, coordinationError(coordination.ErrInvalidArgument, "logger is required")
	}
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, coordinationError(coordination.ErrInvalidArgument, "postgres url is required")
	}
	cfg.normalize()
	if !validTableName.MatchString(cfg.Table) {
		return nil, coordinationError(coordination.ErrValidation, fmt.Sprintf("invalid postgres lock table name %q", cfg.Table))
	}

	db, err := sql.Open("postgres", cfg.URL)
	if err != nil {
		return nil, errors.Join(coordinationError(coordination.ErrRetryable, "open postgres failed"), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.OperationTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(coordinationError(coordination.ErrRetryable, "ping postgres failed"), err, closeErr)
		}
		return nil, errors.Join(coordinationError(coordination.ErrRetryable, "ping postgres failed"), err)
	}

	provider := &PostgresLockProvider{
		db:     db,
		log:    log,
		config: cfg,
	}
	if err := provider.ensureTable(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return provider, nil
}

func newPostgresLockProviderWithDB(db *sql.DB, cfg PostgresLockProviderConfig, log logger.Logger) (*PostgresLockProvider, error) {
	if db == nil {
		return nil, coordinationError(coordination.ErrInvalidArgument, "db is required")
	}
	if log == nil {
		return nil, coordinationError(coordination.ErrInvalidArgument, "logger is required")
	}
	cfg.normalize()
	if !validTableName.MatchString(cfg.Table) {
		return nil, coordinationError(coordination.ErrValidation, fmt.Sprintf("invalid postgres lock table name %q", cfg.Table))
	}
	return &PostgresLockProvider{
		db:     db,
		log:    log,
		config: cfg,
	}, nil
}

// Acquire acquires lock row if missing or expired.
func (p *PostgresLockProvider) Acquire(ctx context.Context, key string, ttl time.Duration) (*coordination.LockLease, bool, error) {
	if p == nil || p.db == nil {
		return nil, false, coordinationError(coordination.ErrNotInitialized, "postgres lock provider is not initialized")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false, coordinationError(coordination.ErrInvalidArgument, "lock key is required")
	}
	if ttl <= 0 {
		return nil, false, coordinationError(coordination.ErrInvalidArgument, "ttl must be > 0")
	}

	token := randomPostgresLockToken()
	opCtx, cancel := p.operationContext(ctx)
	defer cancel()
	expiresAt := time.Now().UTC().Add(ttl)
	tableName, err := p.validatedTableName()
	if err != nil {
		return nil, false, err
	}

	// #nosec G201 -- tableName is constrained by validatedTableName.
	query := fmt.Sprintf(`
WITH upsert AS (
	INSERT INTO %s(lock_key, token, expires_at, updated_at)
	VALUES ($1, $2, $3, NOW())
	ON CONFLICT(lock_key) DO UPDATE
	SET token = EXCLUDED.token,
	    expires_at = EXCLUDED.expires_at,
	    updated_at = NOW()
	WHERE %s.expires_at <= NOW()
	RETURNING 1
)
SELECT EXISTS(SELECT 1 FROM upsert)
`, tableName, tableName)

	var acquired bool
	if err := p.db.QueryRowContext(opCtx, query, key, token, expiresAt).Scan(&acquired); err != nil {
		return nil, false, errors.Join(coordinationError(coordination.ErrRetryable, "acquire lock failed"), err)
	}
	if !acquired {
		return nil, false, nil
	}
	return &coordination.LockLease{
		Key:      key,
		Token:    token,
		ExpireAt: expiresAt,
	}, true, nil
}

// Renew extends lock expiry when token matches.
func (p *PostgresLockProvider) Renew(ctx context.Context, lease *coordination.LockLease, ttl time.Duration) error {
	if p == nil || p.db == nil {
		return coordinationError(coordination.ErrNotInitialized, "postgres lock provider is not initialized")
	}
	if lease == nil {
		return coordinationError(coordination.ErrInvalidArgument, "lease is required")
	}
	if ttl <= 0 {
		return coordinationError(coordination.ErrInvalidArgument, "ttl must be > 0")
	}
	key := strings.TrimSpace(lease.Key)
	token := strings.TrimSpace(lease.Token)
	if key == "" || token == "" {
		return coordinationError(coordination.ErrInvalidArgument, "lease key and token are required")
	}

	opCtx, cancel := p.operationContext(ctx)
	defer cancel()
	tableName, err := p.validatedTableName()
	if err != nil {
		return err
	}
	// #nosec G201 -- tableName is constrained by validatedTableName.
	query := fmt.Sprintf(`UPDATE %s SET expires_at=$3, updated_at=NOW() WHERE lock_key=$1 AND token=$2 AND expires_at > NOW()`, tableName)
	result, err := p.db.ExecContext(opCtx, query, key, token, time.Now().UTC().Add(ttl))
	if err != nil {
		return errors.Join(coordinationError(coordination.ErrRetryable, "renew lock failed"), err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return errors.Join(coordinationError(coordination.ErrRetryable, "renew lock rows affected failed"), err)
	}
	if affected == 0 {
		return coordinationError(coordination.ErrConflict, "lock renew rejected")
	}
	lease.ExpireAt = time.Now().UTC().Add(ttl)
	return nil
}

// Release deletes lock row when token matches.
func (p *PostgresLockProvider) Release(ctx context.Context, lease *coordination.LockLease) error {
	if p == nil || p.db == nil {
		return coordinationError(coordination.ErrNotInitialized, "postgres lock provider is not initialized")
	}
	if lease == nil {
		return coordinationError(coordination.ErrInvalidArgument, "lease is required")
	}
	key := strings.TrimSpace(lease.Key)
	token := strings.TrimSpace(lease.Token)
	if key == "" || token == "" {
		return coordinationError(coordination.ErrInvalidArgument, "lease key and token are required")
	}

	opCtx, cancel := p.operationContext(ctx)
	defer cancel()
	tableName, err := p.validatedTableName()
	if err != nil {
		return err
	}
	// #nosec G201 -- tableName is constrained by validatedTableName.
	query := fmt.Sprintf(`DELETE FROM %s WHERE lock_key=$1 AND token=$2`, tableName)
	result, err := p.db.ExecContext(opCtx, query, key, token)
	if err != nil {
		return errors.Join(coordinationError(coordination.ErrRetryable, "release lock failed"), err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return errors.Join(coordinationError(coordination.ErrRetryable, "release lock rows affected failed"), err)
	}
	if affected == 0 {
		return coordinationError(coordination.ErrConflict, "lock release rejected")
	}
	return nil
}

// HealthCheck verifies Postgres connectivity.
func (p *PostgresLockProvider) HealthCheck(ctx context.Context) error {
	if p == nil || p.db == nil {
		return coordinationError(coordination.ErrNotInitialized, "postgres lock provider is not initialized")
	}
	opCtx, cancel := p.operationContext(ctx)
	defer cancel()
	if err := p.db.PingContext(opCtx); err != nil {
		return errors.Join(coordinationError(coordination.ErrRetryable, "postgres healthcheck failed"), err)
	}
	return nil
}

// Close closes DB resources.
func (p *PostgresLockProvider) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}

func (p *PostgresLockProvider) ensureTable(ctx context.Context) error {
	tableName, err := p.validatedTableName()
	if err != nil {
		return err
	}
	// #nosec G201 -- tableName is constrained by validatedTableName.
	query := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	lock_key TEXT PRIMARY KEY,
	token TEXT NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`, tableName)
	if _, err := p.db.ExecContext(ctx, query); err != nil {
		return errors.Join(coordinationError(coordination.ErrRetryable, "ensure lock table failed"), err)
	}
	return nil
}

func (p *PostgresLockProvider) validatedTableName() (string, error) {
	if !validTableName.MatchString(p.config.Table) {
		return "", coordinationError(coordination.ErrValidation, fmt.Sprintf("invalid postgres lock table name %q", p.config.Table))
	}
	return p.config.Table, nil
}

func (p *PostgresLockProvider) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, p.config.OperationTimeout)
}

func randomPostgresLockToken() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

func messageOrDefault(message, fallback string) string {
	if strings.TrimSpace(message) != "" {
		return message
	}
	return fallback
}
