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
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// Adapter provides OpenSearch/Elasticsearch connectivity.
type Adapter struct {
	baseURLs []url.URL
	client   *http.Client
	logger   logger.Logger
	config   Config

	nextNode uint64
	signer   *v4.Signer
	creds    aws.CredentialsProvider
}

// Config holds OpenSearch/Elasticsearch adapter configuration.
type Config struct {
	URL              string
	URLs             []string
	Username         string
	Password         string
	APIKey           string
	AWSAuthEnabled   bool
	AWSRegion        string
	AWSService       string
	AWSAccessKeyID   string
	AWSSecretKey     string
	AWSSessionToken  string
	MaxConns         int
	OperationTimeout time.Duration
}

// NewAdapter creates a new OpenSearch/Elasticsearch adapter.
func NewAdapter(cfg Config, log logger.Logger) (*Adapter, error) {
	baseURLs, err := parseBaseURLs(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 10
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 5 * time.Second
	}
	if cfg.AWSAuthEnabled {
		if strings.TrimSpace(cfg.AWSRegion) == "" {
			return nil, fmt.Errorf("aws region is required when AWS auth is enabled")
		}
		if strings.TrimSpace(cfg.AWSService) == "" {
			cfg.AWSService = "es"
		}
	}

	transport := &http.Transport{
		MaxIdleConns:        cfg.MaxConns,
		MaxIdleConnsPerHost: cfg.MaxConns,
		MaxConnsPerHost:     cfg.MaxConns,
		IdleConnTimeout:     90 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.OperationTimeout,
	}

	adapter := &Adapter{
		baseURLs: baseURLs,
		client:   client,
		logger:   log,
		config:   cfg,
	}

	if cfg.AWSAuthEnabled {
		if err := adapter.initAWSAuth(); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := adapter.Ping(ctx); err != nil {
		adapter.Close()
		return nil, fmt.Errorf("failed to ping opensearch/elasticsearch: %w", err)
	}

	log.Info("Search connection established",
		"nodes", len(baseURLs),
		"aws_auth_enabled", cfg.AWSAuthEnabled,
		"max_conns", cfg.MaxConns,
		"operation_timeout", cfg.OperationTimeout,
	)
	return adapter, nil
}

// Ping verifies the OpenSearch/Elasticsearch connection is alive.
func (a *Adapter) Ping(ctx context.Context) error {
	resp, err := a.request(ctx, http.MethodGet, "/", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("search ping failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// HealthCheck verifies the OpenSearch/Elasticsearch cluster is healthy.
func (a *Adapter) HealthCheck(ctx context.Context) error {
	hcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := a.request(hcCtx, http.MethodGet, "/_cluster/health?local=true", nil)
	if err != nil {
		a.logger.Error("Search health check failed", "error", err)
		return fmt.Errorf("search health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("search health check failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		a.logger.Error("Search health check failed", "error", err)
		return err
	}
	return nil
}

// IndexDocument upserts a JSON document in the target index by ID.
func (a *Adapter) IndexDocument(ctx context.Context, index, id string, document interface{}) error {
	if strings.TrimSpace(index) == "" {
		return fmt.Errorf("index is required")
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("document id is required")
	}

	payload, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	path := fmt.Sprintf("/%s/_doc/%s", url.PathEscape(index), url.PathEscape(id))
	resp, err := a.request(ctx, http.MethodPut, path, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to index document: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// DeleteDocument deletes a document by ID.
func (a *Adapter) DeleteDocument(ctx context.Context, index, id string) error {
	if strings.TrimSpace(index) == "" {
		return fmt.Errorf("index is required")
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("document id is required")
	}

	path := fmt.Sprintf("/%s/_doc/%s", url.PathEscape(index), url.PathEscape(id))
	resp, err := a.request(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// DELETE is idempotent: not found is acceptable.
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete document: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// Search executes a JSON query and returns the raw JSON response.
func (a *Adapter) Search(ctx context.Context, index string, query interface{}) (json.RawMessage, error) {
	if strings.TrimSpace(index) == "" {
		return nil, fmt.Errorf("index is required")
	}

	payload, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	path := fmt.Sprintf("/%s/_search", url.PathEscape(index))
	resp, err := a.request(ctx, http.MethodPost, path, payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("search request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.RawMessage(body), nil
}

// Close gracefully closes idle HTTP connections.
func (a *Adapter) Close() error {
	a.logger.Info("closing Search connections")
	if transport, ok := a.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	return nil
}

func (a *Adapter) request(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	if len(a.baseURLs) == 0 {
		return nil, fmt.Errorf("no search nodes configured")
	}

	start := int(atomic.AddUint64(&a.nextNode, 1)-1) % len(a.baseURLs)
	var lastErr error

	for attempt := 0; attempt < len(a.baseURLs); attempt++ {
		idx := (start + attempt) % len(a.baseURLs)
		baseURL := a.baseURLs[idx]

		resp, err := a.requestNode(ctx, baseURL, method, path, body)
		if err != nil {
			lastErr = err
			continue
		}

		if shouldRetryOnStatus(resp.StatusCode) && attempt < len(a.baseURLs)-1 {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("node %s returned retryable status %d", baseURL.String(), resp.StatusCode)
			continue
		}
		return resp, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("request failed on all configured nodes")
	}
	return nil, lastErr
}

func (a *Adapter) requestNode(ctx context.Context, baseURL url.URL, method, path string, body []byte) (*http.Response, error) {
	endpoint, err := resolveEndpoint(baseURL, path)
	if err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "nimburion-search-adapter/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if a.config.AWSAuthEnabled {
		if signErr := a.signRequest(ctx, req, body); signErr != nil {
			return nil, signErr
		}
	} else {
		switch {
		case strings.TrimSpace(a.config.APIKey) != "":
			req.Header.Set("Authorization", "ApiKey "+strings.TrimSpace(a.config.APIKey))
		case strings.TrimSpace(a.config.Username) != "":
			req.SetBasicAuth(a.config.Username, a.config.Password)
		}
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", endpoint, err)
	}
	return resp, nil
}

func (a *Adapter) signRequest(ctx context.Context, req *http.Request, body []byte) error {
	if a.signer == nil || a.creds == nil {
		return fmt.Errorf("AWS signer is not initialized")
	}

	creds, err := a.creds.Retrieve(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}

	hash := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(hash[:])
	if err := a.signer.SignHTTP(ctx, creds, req, payloadHash, a.config.AWSService, a.config.AWSRegion, time.Now().UTC()); err != nil {
		return fmt.Errorf("failed to sign request with AWS SigV4: %w", err)
	}
	return nil
}

func (a *Adapter) initAWSAuth() error {
	var provider aws.CredentialsProvider
	if strings.TrimSpace(a.config.AWSAccessKeyID) != "" || strings.TrimSpace(a.config.AWSSecretKey) != "" {
		if strings.TrimSpace(a.config.AWSAccessKeyID) == "" || strings.TrimSpace(a.config.AWSSecretKey) == "" {
			return fmt.Errorf("both AWS access key id and secret access key are required when using static AWS credentials")
		}
		provider = credentials.NewStaticCredentialsProvider(a.config.AWSAccessKeyID, a.config.AWSSecretKey, a.config.AWSSessionToken)
	} else {
		awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(a.config.AWSRegion))
		if err != nil {
			return fmt.Errorf("failed to load AWS default config: %w", err)
		}
		if awsCfg.Credentials == nil {
			return fmt.Errorf("failed to resolve AWS credentials provider")
		}
		provider = awsCfg.Credentials
	}

	a.signer = v4.NewSigner()
	a.creds = provider
	return nil
}

func parseBaseURLs(cfg Config) ([]url.URL, error) {
	raw := make([]string, 0, len(cfg.URLs)+1)
	if strings.TrimSpace(cfg.URL) != "" {
		raw = append(raw, cfg.URL)
	}
	for _, u := range cfg.URLs {
		if strings.TrimSpace(u) != "" {
			raw = append(raw, u)
		}
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("opensearch URL is required (or configure URLs)")
	}

	parsed := make([]url.URL, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		u, err := url.Parse(strings.TrimSpace(item))
		if err != nil {
			return nil, fmt.Errorf("failed to parse search URL %q: %w", item, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("invalid search URL: %s", item)
		}
		key := u.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		parsed = append(parsed, *u)
	}
	return parsed, nil
}

func resolveEndpoint(base url.URL, path string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	rel, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("invalid request path %q: %w", path, err)
	}
	return base.ResolveReference(rel).String(), nil
}

func shouldRetryOnStatus(status int) bool {
	switch status {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
