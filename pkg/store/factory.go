package store

import (
	"fmt"
	"strings"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/store/dynamodb"
	"github.com/nimburion/nimburion/pkg/store/mongodb"
	"github.com/nimburion/nimburion/pkg/store/mysql"
	"github.com/nimburion/nimburion/pkg/store/opensearch"
	"github.com/nimburion/nimburion/pkg/store/postgres"
	"github.com/nimburion/nimburion/pkg/store/s3"
)

// Cosa fa: seleziona e inizializza lo storage adapter in base alla config.
// Cosa NON fa: non gestisce fallback tra provider diversi.
// Esempio minimo: adp, err := store.NewStorageAdapter(cfg.Database, log)
func NewStorageAdapter(cfg config.DatabaseConfig, log logger.Logger) (Adapter, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "postgres":
		return postgres.NewPostgreSQLAdapter(postgres.Config{
			URL:             cfg.URL,
			MaxOpenConns:    cfg.MaxOpenConns,
			MaxIdleConns:    cfg.MaxIdleConns,
			ConnMaxLifetime: cfg.ConnMaxLifetime,
			ConnMaxIdleTime: cfg.ConnMaxIdleTime,
			QueryTimeout:    cfg.QueryTimeout,
		}, log)
	case "mysql":
		return mysql.NewMySQLAdapter(mysql.Config{
			URL:             cfg.URL,
			MaxOpenConns:    cfg.MaxOpenConns,
			MaxIdleConns:    cfg.MaxIdleConns,
			ConnMaxLifetime: cfg.ConnMaxLifetime,
			ConnMaxIdleTime: cfg.ConnMaxIdleTime,
			QueryTimeout:    cfg.QueryTimeout,
		}, log)
	case "mongodb":
		return mongodb.NewMongoDBAdapter(mongodb.Config{
			URL:              cfg.URL,
			Database:         cfg.DatabaseName,
			ConnectTimeout:   cfg.ConnectTimeout,
			OperationTimeout: cfg.QueryTimeout,
		}, log)
	case "dynamodb":
		return dynamodb.NewDynamoDBAdapter(dynamodb.Config{
			Region:           cfg.Region,
			Endpoint:         cfg.Endpoint,
			AccessKeyID:      cfg.AccessKeyID,
			SecretAccessKey:  cfg.SecretAccessKey,
			SessionToken:     cfg.SessionToken,
			OperationTimeout: cfg.QueryTimeout,
		}, log)
	default:
		return nil, fmt.Errorf("unsupported database.type %q (supported: postgres, mysql, mongodb, dynamodb)", cfg.Type)
	}
}

// NewSearchAdapter selects and initializes a search store adapter from config.
func NewSearchAdapter(cfg config.SearchConfig, log logger.Logger) (Adapter, error) {
	searchType := strings.ToLower(strings.TrimSpace(cfg.Type))
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "" {
		driver = "http"
	}

	switch driver {
	case "http":
		switch searchType {
		case "opensearch", "elasticsearch":
			return opensearch.NewOpenSearchAdapter(opensearch.Config{
				URL:              cfg.URL,
				URLs:             cfg.URLs,
				Username:         cfg.Username,
				Password:         cfg.Password,
				APIKey:           cfg.APIKey,
				AWSAuthEnabled:   cfg.AWSAuthEnabled,
				AWSRegion:        cfg.AWSRegion,
				AWSService:       cfg.AWSService,
				AWSAccessKeyID:   cfg.AWSAccessKeyID,
				AWSSecretKey:     cfg.AWSSecretKey,
				AWSSessionToken:  cfg.AWSSessionToken,
				MaxConns:         cfg.MaxConns,
				OperationTimeout: cfg.OperationTimeout,
			}, log)
		default:
			return nil, fmt.Errorf("unsupported search.type %q (supported: opensearch, elasticsearch)", cfg.Type)
		}
	case "opensearch-sdk":
		if searchType != "opensearch" {
			return nil, fmt.Errorf("search.driver %q requires search.type opensearch", cfg.Driver)
		}
		return opensearch.NewOpenSearchSDKAdapter(opensearch.Config{
			URL:              cfg.URL,
			URLs:             cfg.URLs,
			Username:         cfg.Username,
			Password:         cfg.Password,
			APIKey:           cfg.APIKey,
			AWSAuthEnabled:   cfg.AWSAuthEnabled,
			AWSRegion:        cfg.AWSRegion,
			AWSService:       cfg.AWSService,
			AWSAccessKeyID:   cfg.AWSAccessKeyID,
			AWSSecretKey:     cfg.AWSSecretKey,
			AWSSessionToken:  cfg.AWSSessionToken,
			MaxConns:         cfg.MaxConns,
			OperationTimeout: cfg.OperationTimeout,
		}, log)
	case "elasticsearch-sdk":
		if searchType != "elasticsearch" {
			return nil, fmt.Errorf("search.driver %q requires search.type elasticsearch", cfg.Driver)
		}
		return opensearch.NewElasticsearchSDKAdapter(opensearch.Config{
			URL:              cfg.URL,
			URLs:             cfg.URLs,
			Username:         cfg.Username,
			Password:         cfg.Password,
			APIKey:           cfg.APIKey,
			AWSAuthEnabled:   cfg.AWSAuthEnabled,
			AWSRegion:        cfg.AWSRegion,
			AWSService:       cfg.AWSService,
			AWSAccessKeyID:   cfg.AWSAccessKeyID,
			AWSSecretKey:     cfg.AWSSecretKey,
			AWSSessionToken:  cfg.AWSSessionToken,
			MaxConns:         cfg.MaxConns,
			OperationTimeout: cfg.OperationTimeout,
		}, log)
	default:
		return nil, fmt.Errorf("unsupported search.driver %q (supported: http, opensearch-sdk, elasticsearch-sdk)", cfg.Driver)
	}
}

// NewObjectStorageAdapter selects and initializes an object storage adapter from config.
func NewObjectStorageAdapter(cfg config.ObjectStorageConfig, log logger.Logger) (Adapter, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	storageType := strings.ToLower(strings.TrimSpace(cfg.Type))
	if storageType == "" {
		storageType = "s3"
	}

	switch storageType {
	case "s3":
		return s3.NewS3Adapter(s3.Config{
			Bucket:           cfg.S3.Bucket,
			Region:           cfg.S3.Region,
			Endpoint:         cfg.S3.Endpoint,
			AccessKeyID:      cfg.S3.AccessKeyID,
			SecretAccessKey:  cfg.S3.SecretAccessKey,
			SessionToken:     cfg.S3.SessionToken,
			UsePathStyle:     cfg.S3.UsePathStyle,
			OperationTimeout: cfg.S3.OperationTimeout,
			PresignExpiry:    cfg.S3.PresignExpiry,
		}, log)
	default:
		return nil, fmt.Errorf("unsupported object_storage.type %q (supported: s3)", cfg.Type)
	}
}
