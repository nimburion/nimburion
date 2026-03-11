// Package mailgun provides an email provider backed by the Mailgun API.
package mailgun

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nimburion/nimburion/internal/emailkit"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/email"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// Config configures the Mailgun email provider.
type Config = emailconfig.MailgunConfig

// Provider sends email through the Mailgun API.
type Provider struct {
	cfg        Config
	httpClient *http.Client
	log        logger.Logger
}

// New constructs a Mailgun-backed email provider.
func New(cfg Config, log logger.Logger) (*Provider, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, coreerrors.NewValidationWithCode("validation.email.mailgun.token.required", "mailgun token is required", nil, nil)
	}
	if strings.TrimSpace(cfg.Domain) == "" {
		return nil, coreerrors.NewValidationWithCode("validation.email.mailgun.domain.required", "mailgun domain is required", nil, nil)
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.mailgun.net"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &Provider{cfg: cfg, httpClient: emailkit.DefaultHTTPClient(nil, cfg.OperationTimeout), log: log}, nil
}

// Send delivers message using the configured Mailgun account.
func (p *Provider) Send(ctx context.Context, message email.Message) error {
	msg := message.Normalized()
	msg, err := email.ApplyDefaultSender(msg, p.cfg.From)
	if err != nil {
		return err
	}
	if validationErr := msg.Validate(); validationErr != nil {
		return validationErr
	}
	form := url.Values{}
	form.Set("from", msg.From)
	form.Set("to", strings.Join(msg.To, ","))
	if len(msg.Cc) > 0 {
		form.Set("cc", strings.Join(msg.Cc, ","))
	}
	if len(msg.Bcc) > 0 {
		form.Set("bcc", strings.Join(msg.Bcc, ","))
	}
	form.Set("subject", msg.Subject)
	if strings.TrimSpace(msg.TextBody) != "" {
		form.Set("text", msg.TextBody)
	}
	if strings.TrimSpace(msg.HTMLBody) != "" {
		form.Set("html", msg.HTMLBody)
	}
	if msg.ReplyTo != "" {
		form.Set("h:Reply-To", msg.ReplyTo)
	}
	cctx, cancel := emailkit.WithTimeout(ctx, p.cfg.OperationTimeout)
	defer cancel()
	endpoint := strings.TrimRight(p.cfg.BaseURL, "/") + "/v3/" + p.cfg.Domain + "/messages"
	if validationErr := emailkit.ValidateEndpointURL(endpoint); validationErr != nil {
		return validationErr
	}
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth("api", p.cfg.Token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// #nosec G704 -- endpoint is derived from validated BaseURL and checked with ValidateEndpointURL.
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { emailkit.IgnoreCloseError(resp.Body.Close()) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return coreerrors.NewUnavailable(fmt.Sprintf("mailgun send failed with status %d", resp.StatusCode), nil).
			WithDetails(map[string]interface{}{"provider": "mailgun", "status_code": resp.StatusCode})
	}
	return nil
}

// Close releases provider resources.
func (p *Provider) Close() error { return nil }
