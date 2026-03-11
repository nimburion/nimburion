package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// Config configures Server-Sent Events runtime behavior.
type Config struct {
	Enabled               bool           `mapstructure:"enabled"`
	Endpoint              string         `mapstructure:"endpoint"`
	Store                 string         `mapstructure:"store"`
	Bus                   string         `mapstructure:"bus"`
	ReplayLimit           int            `mapstructure:"replay_limit"`
	ClientBuffer          int            `mapstructure:"client_buffer"`
	MaxConnections        int            `mapstructure:"max_connections"`
	HeartbeatInterval     time.Duration  `mapstructure:"heartbeat_interval"`
	DefaultRetryMS        int            `mapstructure:"default_retry_ms"`
	DropOnBackpressure    bool           `mapstructure:"drop_on_backpressure"`
	ChannelQueryParam     string         `mapstructure:"channel_query_param"`
	TenantQueryParam      string         `mapstructure:"tenant_query_param"`
	SubjectQueryParam     string         `mapstructure:"subject_query_param"`
	LastEventIDQueryParam string         `mapstructure:"last_event_id_query_param"`
	Redis                 RedisConfig    `mapstructure:"redis"`
	EventBus              EventBusConfig `mapstructure:"eventbus"`
}

// RedisConfig configures Redis-backed replay store and fan-out.
type RedisConfig struct {
	URL              string        `mapstructure:"url"`
	MaxConns         int           `mapstructure:"max_conns"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
	HistoryPrefix    string        `mapstructure:"history_prefix"`
	PubSubPrefix     string        `mapstructure:"pubsub_prefix"`
}

// EventBusConfig configures event-bus-backed SSE fan-out.
type EventBusConfig struct {
	TopicPrefix      string        `mapstructure:"topic_prefix"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// Extension contributes the SSE config section as family-owned config surface.
type Extension struct {
	SSE Config `mapstructure:"sse"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"sse"} }

// ApplyDefaults registers default SSE configuration values.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("sse.enabled", false)
	v.SetDefault("sse.endpoint", "/events")
	v.SetDefault("sse.store", "inmemory")
	v.SetDefault("sse.bus", "none")
	v.SetDefault("sse.replay_limit", 100)
	v.SetDefault("sse.client_buffer", 64)
	v.SetDefault("sse.max_connections", 10000)
	v.SetDefault("sse.heartbeat_interval", 20*time.Second)
	v.SetDefault("sse.default_retry_ms", 3000)
	v.SetDefault("sse.drop_on_backpressure", true)
	v.SetDefault("sse.channel_query_param", "channel")
	v.SetDefault("sse.tenant_query_param", "tenant")
	v.SetDefault("sse.subject_query_param", "subject")
	v.SetDefault("sse.last_event_id_query_param", "last_event_id")
	v.SetDefault("sse.redis.max_conns", 10)
	v.SetDefault("sse.redis.operation_timeout", 3*time.Second)
	v.SetDefault("sse.redis.history_prefix", "sse:history")
	v.SetDefault("sse.redis.pubsub_prefix", "sse:bus")
	v.SetDefault("sse.eventbus.topic_prefix", "sse")
	v.SetDefault("sse.eventbus.operation_timeout", 5*time.Second)
}

// BindEnv binds SSE configuration keys to environment variables.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"sse.enabled", "SSE_ENABLED",
		"sse.endpoint", "SSE_ENDPOINT",
		"sse.store", "SSE_STORE",
		"sse.bus", "SSE_BUS",
		"sse.replay_limit", "SSE_REPLAY_LIMIT",
		"sse.client_buffer", "SSE_CLIENT_BUFFER",
		"sse.max_connections", "SSE_MAX_CONNECTIONS",
		"sse.heartbeat_interval", "SSE_HEARTBEAT_INTERVAL",
		"sse.default_retry_ms", "SSE_DEFAULT_RETRY_MS",
		"sse.drop_on_backpressure", "SSE_DROP_ON_BACKPRESSURE",
		"sse.channel_query_param", "SSE_CHANNEL_QUERY_PARAM",
		"sse.tenant_query_param", "SSE_TENANT_QUERY_PARAM",
		"sse.subject_query_param", "SSE_SUBJECT_QUERY_PARAM",
		"sse.last_event_id_query_param", "SSE_LAST_EVENT_ID_QUERY_PARAM",
		"sse.redis.url", "SSE_REDIS_URL",
		"sse.redis.max_conns", "SSE_REDIS_MAX_CONNS",
		"sse.redis.operation_timeout", "SSE_REDIS_OPERATION_TIMEOUT",
		"sse.redis.history_prefix", "SSE_REDIS_HISTORY_PREFIX",
		"sse.redis.pubsub_prefix", "SSE_REDIS_PUBSUB_PREFIX",
		"sse.eventbus.topic_prefix", "SSE_EVENTBUS_TOPIC_PREFIX",
		"sse.eventbus.operation_timeout", "SSE_EVENTBUS_OPERATION_TIMEOUT",
	)
}

// Validate checks that enabled SSE configuration is coherent.
func (e Extension) Validate() error {
	if !e.SSE.Enabled {
		return nil
	}
	validStores := []string{"inmemory", "redis"}
	store := strings.ToLower(strings.TrimSpace(e.SSE.Store))
	if !contains(validStores, store) {
		return validationErrorf("validation.sse.store.invalid", "invalid sse.store: %s (must be one of: %v)", e.SSE.Store, validStores)
	}
	validBuses := []string{"none", "inmemory", "redis", "eventbus"}
	bus := strings.ToLower(strings.TrimSpace(e.SSE.Bus))
	if !contains(validBuses, bus) {
		return validationErrorf("validation.sse.bus.invalid", "invalid sse.bus: %s (must be one of: %v)", e.SSE.Bus, validBuses)
	}
	if strings.TrimSpace(e.SSE.Endpoint) == "" || !strings.HasPrefix(strings.TrimSpace(e.SSE.Endpoint), "/") {
		return validationError("validation.sse.endpoint.invalid", "sse.endpoint must be a non-empty absolute path")
	}
	if e.SSE.ReplayLimit <= 0 {
		return validationError("validation.sse.replay_limit.invalid", "sse.replay_limit must be greater than zero when sse is enabled")
	}
	if e.SSE.ClientBuffer <= 0 {
		return validationError("validation.sse.client_buffer.invalid", "sse.client_buffer must be greater than zero when sse is enabled")
	}
	if e.SSE.MaxConnections <= 0 {
		return validationError("validation.sse.max_connections.invalid", "sse.max_connections must be greater than zero when sse is enabled")
	}
	if e.SSE.HeartbeatInterval <= 0 {
		return validationError("validation.sse.heartbeat_interval.invalid", "sse.heartbeat_interval must be greater than zero when sse is enabled")
	}
	if e.SSE.DefaultRetryMS <= 0 {
		return validationError("validation.sse.default_retry_ms.invalid", "sse.default_retry_ms must be greater than zero when sse is enabled")
	}
	if (store == "redis" || bus == "redis") && strings.TrimSpace(e.SSE.Redis.URL) == "" {
		return validationError("validation.sse.redis.url.required", "sse.redis.url is required when sse.store=redis or sse.bus=redis")
	}
	return nil
}

func validationError(code, message string) error {
	return coreerrors.NewValidationWithCode(code, message, nil, nil)
}

func validationErrorf(code, format string, args ...any) error {
	return validationError(code, fmt.Sprintf(format, args...))
}

func bindEnvPairs(v *viper.Viper, prefix string, values ...string) error {
	if len(values)%2 != 0 {
		return fmt.Errorf("bindEnvPairs requires even number of values, got %d", len(values))
	}
	for len(values) > 0 {
		key, suffix := values[0], values[1]
		if err := v.BindEnv(key, prefixedEnv(prefix, suffix)); err != nil {
			return err
		}
		values = values[2:]
	}
	return nil
}

func prefixedEnv(prefix, suffix string) string {
	if strings.TrimSpace(prefix) == "" {
		return suffix
	}
	return strings.TrimSpace(prefix) + "_" + suffix
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
