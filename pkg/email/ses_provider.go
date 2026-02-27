package email

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsv2config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// SESConfig configures AWS SES v2 provider.
type SESConfig struct {
	Region           string
	From             string
	Endpoint         string
	AccessKeyID      string
	SecretAccessKey  string
	SessionToken     string
	OperationTimeout time.Duration
	HTTPClient       *http.Client
}

// SESProvider sends email through AWS SES v2 API.
type SESProvider struct {
	cfg        SESConfig
	awsCfg     awsv2.Config
	signer     *v4.Signer
	httpClient *http.Client
	log        logger.Logger
}

// NewSESProvider creates a SES adapter.
func NewSESProvider(cfg SESConfig, log logger.Logger) (*SESProvider, error) {
	if strings.TrimSpace(cfg.Region) == "" {
		return nil, fmt.Errorf("ses region is required")
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}

	loadOpts := []func(*awsv2config.LoadOptions) error{
		awsv2config.WithRegion(cfg.Region),
	}
	if strings.TrimSpace(cfg.AccessKeyID) != "" || strings.TrimSpace(cfg.SecretAccessKey) != "" {
		loadOpts = append(loadOpts, awsv2config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		))
	}
	awsCfg, err := awsv2config.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, err
	}
	return &SESProvider{
		cfg:        cfg,
		awsCfg:     awsCfg,
		signer:     v4.NewSigner(),
		httpClient: defaultHTTPClient(cfg.HTTPClient, cfg.OperationTimeout),
		log:        log,
	}, nil
}

// Send sends email via SES v2 HTTPS API.
func (p *SESProvider) Send(ctx context.Context, message Message) error {
	msg := message.normalized()
	msg, err := applyDefaultSender(msg, p.cfg.From)
	if err != nil {
		return err
	}
	if validateErr := msg.validate(); validateErr != nil {
		return validateErr
	}

	payload := map[string]interface{}{
		"FromEmailAddress": msg.From,
		"Destination": map[string]interface{}{
			"ToAddresses":  msg.To,
			"CcAddresses":  msg.Cc,
			"BccAddresses": msg.Bcc,
		},
		"Content": map[string]interface{}{
			"Simple": map[string]interface{}{
				"Subject": map[string]string{"Data": msg.Subject},
				"Body": map[string]interface{}{
					"Text": map[string]string{"Data": msg.TextBody},
					"Html": map[string]string{"Data": msg.HTMLBody},
				},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := strings.TrimSpace(p.cfg.Endpoint)
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://email.%s.amazonaws.com", p.cfg.Region)
	}

	cctx, cancel := withTimeout(ctx, p.cfg.OperationTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(cctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/v2/email/outbound-emails", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	hash := sha256.Sum256(raw)
	payloadHash := hex.EncodeToString(hash[:])
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	creds, err := p.awsCfg.Credentials.Retrieve(cctx)
	if err != nil {
		return err
	}
	if signErr := p.signer.SignHTTP(cctx, creds, req, payloadHash, "ses", p.cfg.Region, time.Now().UTC()); signErr != nil {
		return signErr
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ses send failed with status %d", resp.StatusCode)
	}
	return nil
}

// Close releases resources.
func (p *SESProvider) Close() error {
	return nil
}
