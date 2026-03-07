package config

import (
	"reflect"
	"strings"
	"testing"
	"time"

	authconfig "github.com/nimburion/nimburion/pkg/auth/config"
	cacheconfig "github.com/nimburion/nimburion/pkg/cache/config"
	eventbusconfig "github.com/nimburion/nimburion/pkg/eventbus/config"
	serverconfig "github.com/nimburion/nimburion/pkg/http/server/config"
	jobsconfig "github.com/nimburion/nimburion/pkg/jobs/config"
	persistenceconfig "github.com/nimburion/nimburion/pkg/persistence/config"
	schedulerconfig "github.com/nimburion/nimburion/pkg/scheduler/config"
)

func TestConfigValidate_Rules(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "management auth requires auth enabled",
			cfg: Config{
				Management: serverconfig.ManagementConfig{AuthEnabled: true},
				Auth:       authconfig.Config{Enabled: false},
			},
			wantErr: "auth.enabled must be true when management.auth_enabled is true",
		},
		{
			name: "postgres requires database url",
			cfg: Config{
				Database: persistenceconfig.DatabaseConfig{Type: persistenceconfig.DatabaseTypePostgres},
			},
			wantErr: "database.url is required when database.type is specified",
		},
		{
			name: "mongodb requires database name",
			cfg: Config{
				Database: persistenceconfig.DatabaseConfig{
					Type: persistenceconfig.DatabaseTypeMongoDB,
					URL:  "mongodb://localhost:27017",
				},
			},
			wantErr: "database.database_name is required when database.type is mongodb",
		},
		{
			name: "dynamodb requires region",
			cfg: Config{
				Database: persistenceconfig.DatabaseConfig{
					Type: persistenceconfig.DatabaseTypeDynamoDB,
				},
			},
			wantErr: "database.region is required when database.type is dynamodb",
		},
		{
			name: "redis cache requires url",
			cfg: Config{
				Cache: cacheconfig.Config{Type: "redis"},
			},
			wantErr: "cache.url is required when cache.type is redis",
		},
		{
			name: "kafka eventbus requires brokers",
			cfg: Config{
				EventBus: eventbusconfig.Config{Type: eventbusconfig.EventBusTypeKafka},
			},
			wantErr: "eventbus.brokers is required when eventbus.type is specified",
		},
		{
			name: "valid dynamodb and sqs config",
			cfg: Config{
				Database: persistenceconfig.DatabaseConfig{
					Type:   persistenceconfig.DatabaseTypeDynamoDB,
					Region: "eu-west-1",
				},
				EventBus: eventbusconfig.Config{
					Type:     eventbusconfig.EventBusTypeSQS,
					Region:   "eu-west-1",
					QueueURL: "https://sqs.eu-west-1.amazonaws.com/123/queue",
				},
				Jobs: jobsconfig.Config{
					Backend: jobsconfig.BackendEventBus,
				},
			},
			wantErr: "",
		},
		{
			name: "invalid jobs backend",
			cfg: Config{
				Jobs: jobsconfig.Config{Backend: "invalid"},
			},
			wantErr: "invalid jobs.backend",
		},
		{
			name: "valid redis jobs backend",
			cfg: Config{
				Jobs: jobsconfig.Config{
					Backend: jobsconfig.BackendRedis,
					Redis: jobsconfig.RedisConfig{
						URL:    "redis://localhost:6379",
						Prefix: "jobs:test",
					},
				},
			},
			wantErr: "",
		},
		{
			name: "scheduler redis requires connection url",
			cfg: Config{
				Scheduler: schedulerconfig.Config{
					Enabled:         true,
					LockProvider:    schedulerconfig.LockProviderRedis,
					LockTTL:         time.Second,
					DispatchTimeout: time.Second,
					Redis: schedulerconfig.RedisConfig{
						Prefix:           "scheduler:locks",
						OperationTimeout: time.Second,
					},
				},
			},
			wantErr: "scheduler.redis.url (or cache.url) is required",
		},
		{
			name: "valid scheduler postgres config",
			cfg: Config{
				Database: persistenceconfig.DatabaseConfig{
					URL: "postgres://localhost:5432/app?sslmode=disable",
				},
				Scheduler: schedulerconfig.Config{
					Enabled:         true,
					LockProvider:    schedulerconfig.LockProviderPostgres,
					LockTTL:         time.Second,
					DispatchTimeout: time.Second,
					Postgres: schedulerconfig.PostgresConfig{
						Table:            "scheduler_locks",
						OperationTimeout: time.Second,
					},
				},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			mergeConfigForTest(t, cfg, tt.cfg)
			err := NewViperLoader("", "APP").Validate(cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func mergeConfigForTest(t *testing.T, dst *Config, overlay Config) {
	t.Helper()
	mergeValue(reflect.ValueOf(dst).Elem(), reflect.ValueOf(overlay))
}

func mergeValue(dst, src reflect.Value) {
	if !src.IsValid() || isZeroValue(src) {
		return
	}

	switch src.Kind() {
	case reflect.Struct:
		for i := 0; i < src.NumField(); i++ {
			mergeValue(dst.Field(i), src.Field(i))
		}
	case reflect.Pointer:
		if src.IsNil() {
			return
		}
		if dst.IsNil() {
			dst.Set(reflect.New(src.Elem().Type()))
		}
		mergeValue(dst.Elem(), src.Elem())
	case reflect.Map, reflect.Slice:
		if src.Len() == 0 {
			return
		}
		dst.Set(src)
	default:
		if dst.CanSet() {
			dst.Set(src)
		}
	}
}

func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !isZeroValue(v.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Map, reflect.Slice:
		return v.IsNil() || v.Len() == 0
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	default:
		return v.IsZero()
	}
}
