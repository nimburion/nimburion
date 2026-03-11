// Package postmark provides an email provider backed by the Postmark API.
package postmark

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
	// Config configures the Postmark email provider.
	Config = emailconfig.PostmarkConfig
	// Provider sends email through the Postmark API.
	Provider struct {
		cfg        Config
		httpClient *http.Client
		log        logger.Logger
	}
)

// New constructs a Postmark-backed email provider.
func New(cfg Config, log logger.Logger) (*Provider, error) {
	if strings.TrimSpace(cfg.ServerToken) == "" {
		return nil, coreerrors.NewValidationWithCode("validation.email.postmark.server_token.required", "postmark server token is required", nil, nil)
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.postmarkapp.com"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &Provider{cfg: cfg, httpClient: emailkit.DefaultHTTPClient(nil, cfg.OperationTimeout), log: log}, nil
}

// Send delivers message using the configured Postmark account.
func (p *Provider) Send(ctx context.Context, message email.Message) error {
	msg, err := email.ApplyDefaultSender(message.Normalized(), p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.Validate(); err != nil {
		return err
	}
	payload := map[string]interface{}{"From": msg.From, "To": strings.Join(msg.To, ","), "Cc": strings.Join(msg.Cc, ","), "Bcc": strings.Join(msg.Bcc, ","), "Subject": msg.Subject, "TextBody": msg.TextBody, "HtmlBody": msg.HTMLBody, "ReplyTo": msg.ReplyTo}
	return emailkit.SendJSON(ctx, p.httpClient, p.cfg.OperationTimeout, strings.TrimRight(p.cfg.BaseURL, "/")+"/email", payload, map[string]string{"X-Postmark-Server-Token": p.cfg.ServerToken})
}

// Close releases provider resources.
func (p *Provider) Close() error { return nil }
