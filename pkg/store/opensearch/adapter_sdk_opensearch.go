//go:build opensearch_sdk

package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	opensearchsdk "github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/signer"
	awssigner "github.com/opensearch-project/opensearch-go/v4/signer/awsv2"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// OpenSearchSDKAdapter is an adapter backed by the official OpenSearch Go client.
type OpenSearchSDKAdapter struct {
	client    *opensearchsdk.Client
	logger    logger.Logger
	transport *http.Transport
}

// NewOpenSearchSDKAdapter creates an OpenSearch adapter backed by the official SDK.
func NewOpenSearchSDKAdapter(cfg Config, log logger.Logger) (*OpenSearchSDKAdapter, error) {
	addresses, err := collectAddresses(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 5 * time.Second
	}
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 10
	}

	transport := &http.Transport{
		MaxIdleConns:        cfg.MaxConns,
		MaxIdleConnsPerHost: cfg.MaxConns,
		MaxConnsPerHost:     cfg.MaxConns,
		IdleConnTimeout:     90 * time.Second,
	}

	clientCfg := opensearchsdk.Config{
		Addresses: addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
		Transport: transport,
	}
	if strings.TrimSpace(cfg.APIKey) != "" {
		clientCfg.Header = http.Header{"Authorization": []string{"ApiKey " + strings.TrimSpace(cfg.APIKey)}}
	}
	if cfg.AWSAuthEnabled {
		signer, err := newOpenSearchAWSSigner(cfg)
		if err != nil {
			return nil, err
		}
		clientCfg.Signer = signer
	}

	client, err := opensearchsdk.NewClient(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create opensearch sdk client: %w", err)
	}

	adapter := &OpenSearchSDKAdapter{client: client, logger: log, transport: transport}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := adapter.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping opensearch via sdk: %w", err)
	}
	return adapter, nil
}

func (a *OpenSearchSDKAdapter) Ping(ctx context.Context) error {
	resp, err := a.perform(ctx, http.MethodGet, "/", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("opensearch sdk ping failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (a *OpenSearchSDKAdapter) HealthCheck(ctx context.Context) error {
	resp, err := a.perform(ctx, http.MethodGet, "/_cluster/health?local=true", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("opensearch sdk health check failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (a *OpenSearchSDKAdapter) IndexDocument(ctx context.Context, index, id string, document interface{}) error {
	payload, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}
	resp, err := a.perform(ctx, http.MethodPut, fmt.Sprintf("/%s/_doc/%s", index, id), payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("opensearch sdk index failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (a *OpenSearchSDKAdapter) DeleteDocument(ctx context.Context, index, id string) error {
	resp, err := a.perform(ctx, http.MethodDelete, fmt.Sprintf("/%s/_doc/%s", index, id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("opensearch sdk delete failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (a *OpenSearchSDKAdapter) Search(ctx context.Context, index string, query interface{}) (json.RawMessage, error) {
	payload, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}
	resp, err := a.perform(ctx, http.MethodPost, fmt.Sprintf("/%s/_search", index), payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("opensearch sdk search failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.RawMessage(body), nil
}

func (a *OpenSearchSDKAdapter) Close() error {
	if a.transport != nil {
		a.transport.CloseIdleConnections()
	}
	return nil
}

func (a *OpenSearchSDKAdapter) perform(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, path, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Perform(req)
	if err != nil {
		return nil, fmt.Errorf("opensearch sdk request failed: %w", err)
	}
	return resp, nil
}

func newOpenSearchAWSSigner(cfg Config) (signer.Signer, error) {
	if strings.TrimSpace(cfg.AWSRegion) == "" {
		return nil, fmt.Errorf("aws region is required when AWS auth is enabled")
	}
	if strings.TrimSpace(cfg.AWSService) == "" {
		cfg.AWSService = "es"
	}

	var awsCfg aws.Config
	if strings.TrimSpace(cfg.AWSAccessKeyID) != "" || strings.TrimSpace(cfg.AWSSecretKey) != "" {
		if strings.TrimSpace(cfg.AWSAccessKeyID) == "" || strings.TrimSpace(cfg.AWSSecretKey) == "" {
			return nil, fmt.Errorf("both AWS access key id and secret access key are required when using static AWS credentials")
		}
		awsCfg = aws.Config{
			Region:      cfg.AWSRegion,
			Credentials: credentials.NewStaticCredentialsProvider(cfg.AWSAccessKeyID, cfg.AWSSecretKey, cfg.AWSSessionToken),
		}
	} else {
		loaded, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(cfg.AWSRegion))
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
		awsCfg = loaded
	}

	return awssigner.NewSignerWithService(awsCfg, cfg.AWSService)
}
