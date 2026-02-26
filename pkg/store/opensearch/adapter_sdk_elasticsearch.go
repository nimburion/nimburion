//go:build elasticsearch_sdk

package opensearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	elasticsearch "github.com/elastic/go-elasticsearch/v8"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// ElasticsearchSDKAdapter is an adapter backed by the official Elasticsearch Go client.
type ElasticsearchSDKAdapter struct {
	client    *elasticsearch.Client
	logger    logger.Logger
	transport *http.Transport
}

// NewElasticsearchSDKAdapter creates an Elasticsearch adapter backed by the official SDK.
func NewElasticsearchSDKAdapter(cfg Config, log logger.Logger) (*ElasticsearchSDKAdapter, error) {
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

	baseTransport := &http.Transport{
		MaxIdleConns:        cfg.MaxConns,
		MaxIdleConnsPerHost: cfg.MaxConns,
		MaxConnsPerHost:     cfg.MaxConns,
		IdleConnTimeout:     90 * time.Second,
	}
	transport := http.RoundTripper(baseTransport)
	if cfg.AWSAuthEnabled {
		awsSigner, creds, err := newElasticAWSSigner(cfg)
		if err != nil {
			return nil, err
		}
		transport = &awsSigningRoundTripper{
			base:    baseTransport,
			signer:  awsSigner,
			creds:   creds,
			region:  cfg.AWSRegion,
			service: cfg.AWSService,
		}
	}

	clientCfg := elasticsearch.Config{
		Addresses: addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
		APIKey:    cfg.APIKey,
		Transport: transport,
	}
	client, err := elasticsearch.NewClient(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch sdk client: %w", err)
	}

	adapter := &ElasticsearchSDKAdapter{
		client:    client,
		logger:    log,
		transport: baseTransport,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := adapter.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping elasticsearch via sdk: %w", err)
	}
	return adapter, nil
}

// Ping performs a basic connectivity check to verify the service is reachable.
func (a *ElasticsearchSDKAdapter) Ping(ctx context.Context) error {
	resp, err := a.perform(ctx, http.MethodGet, "/", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("elasticsearch sdk ping failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// HealthCheck verifies the component is operational and can perform its intended function.
func (a *ElasticsearchSDKAdapter) HealthCheck(ctx context.Context) error {
	resp, err := a.perform(ctx, http.MethodGet, "/_cluster/health?local=true", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("elasticsearch sdk health check failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// IndexDocument TODO: add description
func (a *ElasticsearchSDKAdapter) IndexDocument(ctx context.Context, index, id string, document interface{}) error {
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
		return fmt.Errorf("elasticsearch sdk index failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// DeleteDocument TODO: add description
func (a *ElasticsearchSDKAdapter) DeleteDocument(ctx context.Context, index, id string) error {
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
		return fmt.Errorf("elasticsearch sdk delete failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// Search TODO: add description
func (a *ElasticsearchSDKAdapter) Search(ctx context.Context, index string, query interface{}) (json.RawMessage, error) {
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
		return nil, fmt.Errorf("elasticsearch sdk search failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.RawMessage(body), nil
}

// Close releases all resources held by this instance. Should be called when the instance is no longer needed.
func (a *ElasticsearchSDKAdapter) Close() error {
	if a.transport != nil {
		a.transport.CloseIdleConnections()
	}
	return nil
}

func (a *ElasticsearchSDKAdapter) perform(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
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
		return nil, fmt.Errorf("elasticsearch sdk request failed: %w", err)
	}
	return resp, nil
}

type awsSigningRoundTripper struct {
	base    http.RoundTripper
	signer  *v4.Signer
	creds   aws.CredentialsProvider
	region  string
	service string
}

// RoundTrip TODO: add description
func (rt *awsSigningRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clonedReq := req.Clone(req.Context())
	payload, err := readRequestBody(clonedReq)
	if err != nil {
		return nil, err
	}

	credentials, err := rt.creds.Retrieve(clonedReq.Context())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}
	hash := sha256.Sum256(payload)
	payloadHash := hex.EncodeToString(hash[:])
	if err := rt.signer.SignHTTP(clonedReq.Context(), credentials, clonedReq, payloadHash, rt.service, rt.region, time.Now().UTC()); err != nil {
		return nil, fmt.Errorf("failed to sign request with AWS SigV4: %w", err)
	}
	return rt.base.RoundTrip(clonedReq)
}

func newElasticAWSSigner(cfg Config) (*v4.Signer, aws.CredentialsProvider, error) {
	if strings.TrimSpace(cfg.AWSRegion) == "" {
		return nil, nil, fmt.Errorf("aws region is required when AWS auth is enabled")
	}
	if strings.TrimSpace(cfg.AWSService) == "" {
		cfg.AWSService = "es"
	}

	if strings.TrimSpace(cfg.AWSAccessKeyID) != "" || strings.TrimSpace(cfg.AWSSecretKey) != "" {
		if strings.TrimSpace(cfg.AWSAccessKeyID) == "" || strings.TrimSpace(cfg.AWSSecretKey) == "" {
			return nil, nil, fmt.Errorf("both AWS access key id and secret access key are required when using static AWS credentials")
		}
		return v4.NewSigner(), credentials.NewStaticCredentialsProvider(cfg.AWSAccessKeyID, cfg.AWSSecretKey, cfg.AWSSessionToken), nil
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	if awsCfg.Credentials == nil {
		return nil, nil, fmt.Errorf("failed to resolve AWS credentials provider")
	}
	return v4.NewSigner(), awsCfg.Credentials, nil
}

func readRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return []byte{}, nil
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(data))
	return data, nil
}
