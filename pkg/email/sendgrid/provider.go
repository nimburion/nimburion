// Package sendgrid provides an email provider backed by the SendGrid API.
package sendgrid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nimburion/nimburion/internal/emailkit"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/email"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// Config configures the SendGrid email provider.
type Config = emailconfig.TokenConfig

// Provider sends email through the SendGrid API.
type Provider struct {
	cfg        Config
	httpClient *http.Client
	log        logger.Logger
}

// New constructs a SendGrid-backed email provider.
func New(cfg Config, log logger.Logger) (*Provider, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, coreerrors.NewValidationWithCode("validation.email.sendgrid.token.required", "sendgrid token is required", nil, nil)
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.sendgrid.com"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &Provider{cfg: cfg, httpClient: emailkit.DefaultHTTPClient(nil, cfg.OperationTimeout), log: log}, nil
}

// Send delivers message using the configured SendGrid account.
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
		"personalizations": []map[string]interface{}{{"to": emailkit.MapRecipients(msg.To), "cc": emailkit.MapRecipients(msg.Cc), "bcc": emailkit.MapRecipients(msg.Bcc)}},
		"from":             map[string]string{"email": msg.From}, "subject": msg.Subject, "content": emailkit.MapContent(msg),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	cctx, cancel := emailkit.WithTimeout(ctx, p.cfg.OperationTimeout)
	defer cancel()
	endpoint := strings.TrimRight(p.cfg.BaseURL, "/") + "/v3/mail/send"
	if validationErr := emailkit.ValidateEndpointURL(endpoint); validationErr != nil {
		return validationErr
	}
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	// #nosec G704 -- endpoint is derived from validated BaseURL and checked with ValidateEndpointURL.
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { emailkit.IgnoreCloseError(resp.Body.Close()) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return coreerrors.NewUnavailable(fmt.Sprintf("sendgrid send failed with status %d", resp.StatusCode), nil).
			WithDetails(map[string]interface{}{"provider": "sendgrid", "status_code": resp.StatusCode})
	}
	return nil
}

// Close releases provider resources.
func (p *Provider) Close() error { return nil }
