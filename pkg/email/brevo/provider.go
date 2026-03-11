// Package brevo provides an email provider backed by the Brevo API.
package brevo

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/nimburion/nimburion/internal/emailkit"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/email"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type (
	// Config configures the Brevo email provider.
	Config = emailconfig.TokenConfig
	// Provider sends email through the Brevo API.
	Provider struct {
		cfg        Config
		httpClient *http.Client
		log        logger.Logger
	}
)

// New constructs a Brevo-backed email provider.
func New(cfg Config, log logger.Logger) (*Provider, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, coreerrors.NewValidationWithCode("validation.email.brevo.token.required", "brevo token is required", nil, nil)
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.brevo.com"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &Provider{cfg: cfg, httpClient: emailkit.DefaultHTTPClient(nil, cfg.OperationTimeout), log: log}, nil
}

// Send delivers message using the configured Brevo account.
func (p *Provider) Send(ctx context.Context, message email.Message) error {
	msg, err := email.ApplyDefaultSender(message.Normalized(), p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.Validate(); err != nil {
		return err
	}
	payload := map[string]interface{}{"sender": map[string]string{"email": msg.From}, "to": emailkit.MapRecipients(msg.To), "cc": emailkit.MapRecipients(msg.Cc), "bcc": emailkit.MapRecipients(msg.Bcc), "subject": msg.Subject, "textContent": msg.TextBody, "htmlContent": msg.HTMLBody}
	return emailkit.SendJSON(ctx, p.httpClient, p.cfg.OperationTimeout, strings.TrimRight(p.cfg.BaseURL, "/")+"/v3/smtp/email", payload, map[string]string{"api-key": p.cfg.Token})
}

// Close releases provider resources.
func (p *Provider) Close() error { return nil }
