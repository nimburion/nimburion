// Package ses provides an email provider backed by AWS SES.
package ses

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

	"github.com/nimburion/nimburion/internal/emailkit"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/email"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// Config configures the SES email provider.
type Config = emailconfig.SESConfig

// Provider sends email through AWS SES.
type Provider struct {
	cfg        Config
	awsCfg     awsv2.Config
	signer     *v4.Signer
	httpClient *http.Client
	log        logger.Logger
}

// New constructs an SES-backed email provider.
func New(cfg Config, log logger.Logger) (*Provider, error) {
	if strings.TrimSpace(cfg.Region) == "" {
		return nil, coreerrors.NewValidationWithCode("validation.email.ses.region.required", "ses region is required", nil, nil)
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	loadOpts := []func(*awsv2config.LoadOptions) error{awsv2config.WithRegion(cfg.Region)}
	if strings.TrimSpace(cfg.AccessKeyID) != "" || strings.TrimSpace(cfg.SecretAccessKey) != "" {
		loadOpts = append(loadOpts, awsv2config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		))
	}
	awsCfg, err := awsv2config.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, err
	}
	return &Provider{cfg: cfg, awsCfg: awsCfg, signer: v4.NewSigner(), httpClient: emailkit.DefaultHTTPClient(nil, cfg.OperationTimeout), log: log}, nil
}

// Send delivers message using the configured SES account.
func (p *Provider) Send(ctx context.Context, message email.Message) error {
	msg := message.Normalized()
	msg, err := email.ApplyDefaultSender(msg, p.cfg.From)
	if err != nil {
		return err
	}
	if validationErr := msg.Validate(); validationErr != nil {
		return validationErr
	}
	payload := map[string]interface{}{
		"FromEmailAddress": msg.From,
		"Destination":      map[string]interface{}{"ToAddresses": msg.To, "CcAddresses": msg.Cc, "BccAddresses": msg.Bcc},
		"Content": map[string]interface{}{"Simple": map[string]interface{}{
			"Subject": map[string]string{"Data": msg.Subject},
			"Body":    map[string]interface{}{"Text": map[string]string{"Data": msg.TextBody}, "Html": map[string]string{"Data": msg.HTMLBody}},
		}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := strings.TrimSpace(p.cfg.Endpoint)
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://email.%s.amazonaws.com", p.cfg.Region)
	}
	endpoint = strings.TrimRight(endpoint, "/") + "/v2/email/outbound-emails"
	if validationErr := emailkit.ValidateEndpointURL(endpoint); validationErr != nil {
		return validationErr
	}
	cctx, cancel := emailkit.WithTimeout(ctx, p.cfg.OperationTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, endpoint, bytes.NewReader(raw))
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
	// #nosec G704 -- endpoint is derived from region/config endpoint and validated with ValidateEndpointURL.
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { emailkit.IgnoreCloseError(resp.Body.Close()) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return coreerrors.NewUnavailable(fmt.Sprintf("ses send failed with status %d", resp.StatusCode), nil).
			WithDetails(map[string]interface{}{"provider": "ses", "status_code": resp.StatusCode})
	}
	return nil
}

// Close releases provider resources.
func (p *Provider) Close() error { return nil }
