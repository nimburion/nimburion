package config

import (
	"strings"
	"testing"
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
				Management: ManagementConfig{AuthEnabled: true},
				Auth:       AuthConfig{Enabled: false},
			},
			wantErr: "management.auth_enabled requires auth.enabled to be true",
		},
		{
			name: "postgres requires database url",
			cfg: Config{
				Database: DatabaseConfig{Type: DatabaseTypePostgres},
			},
			wantErr: "database.url is required when database.type is set",
		},
		{
			name: "mongodb requires database name",
			cfg: Config{
				Database: DatabaseConfig{
					Type: DatabaseTypeMongoDB,
					URL:  "mongodb://localhost:27017",
				},
			},
			wantErr: "database.database_name is required for MongoDB",
		},
		{
			name: "dynamodb requires region",
			cfg: Config{
				Database: DatabaseConfig{
					Type: DatabaseTypeDynamoDB,
				},
			},
			wantErr: "database.region is required for DynamoDB",
		},
		{
			name: "redis cache requires url",
			cfg: Config{
				Cache: CacheConfig{Type: "redis"},
			},
			wantErr: "cache.url is required when cache.type is redis",
		},
		{
			name: "kafka eventbus requires brokers",
			cfg: Config{
				EventBus: EventBusConfig{Type: EventBusTypeKafka},
			},
			wantErr: "eventbus.brokers is required for Kafka",
		},
		{
			name: "valid dynamodb and sqs config",
			cfg: Config{
				Database: DatabaseConfig{
					Type:   DatabaseTypeDynamoDB,
					Region: "eu-west-1",
				},
				EventBus: EventBusConfig{
					Type:     EventBusTypeSQS,
					Region:   "eu-west-1",
					QueueURL: "https://sqs.eu-west-1.amazonaws.com/123/queue",
				},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
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
