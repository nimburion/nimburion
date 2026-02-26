package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

const (
	defaultPostgresLockTable     = "nimburion_scheduler_locks"
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
		return nil, errors.New("logger is required")
	}
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, errors.New("postgres url is required")
	}
	cfg.normalize()
	if !validTableName.MatchString(cfg.Table) {
		return nil, fmt.Errorf("invalid scheduler postgres table name %q", cfg.Table)
	}

	db, err := sql.Open("postgres", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("open postgres failed: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.OperationTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres failed: %w", err)
	}

	provider := &PostgresLockProvider{
		db:     db,
		log:    log,
		config: cfg,
	}
	if err := provider.ensureTable(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return provider, nil
}

func newPostgresLockProviderWithDB(db *sql.DB, cfg PostgresLockProviderConfig, log logger.Logger) (*PostgresLockProvider, error) {
	if db == nil {
		return nil, errors.New("db is required")
	}
	if log == nil {
		return nil, errors.New("logger is required")
	}
	cfg.normalize()
	if !validTableName.MatchString(cfg.Table) {
		return nil, fmt.Errorf("invalid scheduler postgres table name %q", cfg.Table)
	}
	return &PostgresLockProvider{
		db:     db,
		log:    log,
		config: cfg,
	}, nil
}

// Acquire acquires lock row if missing or expired.
func (p *PostgresLockProvider) Acquire(ctx context.Context, key string, ttl time.Duration) (*LockLease, bool, error) {
	if p == nil || p.db == nil {
		return nil, false, errors.New("postgres lock provider is not initialized")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false, errors.New("lock key is required")
	}
	if ttl <= 0 {
		return nil, false, errors.New("ttl must be > 0")
	}

	token := randomSchedulerID()
	opCtx, cancel := p.operationContext(ctx)
	defer cancel()
	expiresAt := time.Now().UTC().Add(ttl)

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
`, p.config.Table, p.config.Table)

	var acquired bool
	if err := p.db.QueryRowContext(opCtx, query, key, token, expiresAt).Scan(&acquired); err != nil {
		return nil, false, err
	}
	if !acquired {
		return nil, false, nil
	}
	return &LockLease{
		Key:      key,
		Token:    token,
		ExpireAt: expiresAt,
	}, true, nil
}

// Renew extends lock expiry when token matches.
func (p *PostgresLockProvider) Renew(ctx context.Context, lease *LockLease, ttl time.Duration) error {
	if p == nil || p.db == nil {
		return errors.New("postgres lock provider is not initialized")
	}
	if lease == nil {
		return errors.New("lease is required")
	}
	if ttl <= 0 {
		return errors.New("ttl must be > 0")
	}
	key := strings.TrimSpace(lease.Key)
	token := strings.TrimSpace(lease.Token)
	if key == "" || token == "" {
		return errors.New("lease key and token are required")
	}

	opCtx, cancel := p.operationContext(ctx)
	defer cancel()
	query := fmt.Sprintf(`UPDATE %s SET expires_at=$3, updated_at=NOW() WHERE lock_key=$1 AND token=$2 AND expires_at > NOW()`, p.config.Table)
	result, err := p.db.ExecContext(opCtx, query, key, token, time.Now().UTC().Add(ttl))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("lock renew rejected")
	}
	lease.ExpireAt = time.Now().UTC().Add(ttl)
	return nil
}

// Release deletes lock row when token matches.
func (p *PostgresLockProvider) Release(ctx context.Context, lease *LockLease) error {
	if p == nil || p.db == nil {
		return errors.New("postgres lock provider is not initialized")
	}
	if lease == nil {
		return errors.New("lease is required")
	}
	key := strings.TrimSpace(lease.Key)
	token := strings.TrimSpace(lease.Token)
	if key == "" || token == "" {
		return errors.New("lease key and token are required")
	}

	opCtx, cancel := p.operationContext(ctx)
	defer cancel()
	query := fmt.Sprintf(`DELETE FROM %s WHERE lock_key=$1 AND token=$2`, p.config.Table)
	result, err := p.db.ExecContext(opCtx, query, key, token)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("lock release rejected")
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
	query := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	lock_key TEXT PRIMARY KEY,
	token TEXT NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`, p.config.Table)
	_, err := p.db.ExecContext(ctx, query)
	return err
}

func (p *PostgresLockProvider) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, p.config.OperationTimeout)
}
