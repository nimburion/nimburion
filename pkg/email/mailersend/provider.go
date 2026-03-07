package mailersend

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nimburion/nimburion/internal/emailkit"
	"github.com/nimburion/nimburion/pkg/email"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type Config = emailconfig.TokenConfig
type Provider struct {
	cfg        Config
	httpClient *http.Client
	log        logger.Logger
}

func New(cfg Config, log logger.Logger) (*Provider, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, fmt.Errorf("mailersend token is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.mailersend.com"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &Provider{cfg: cfg, httpClient: emailkit.DefaultHTTPClient(nil, cfg.OperationTimeout), log: log}, nil
}
func (p *Provider) Send(ctx context.Context, message email.Message) error {
	msg, err := email.ApplyDefaultSender(message.Normalized(), p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.Validate(); err != nil {
		return err
	}
	payload := map[string]interface{}{"from": map[string]string{"email": msg.From}, "to": emailkit.MapRecipients(msg.To), "cc": emailkit.MapRecipients(msg.Cc), "bcc": emailkit.MapRecipients(msg.Bcc), "subject": msg.Subject, "text": msg.TextBody, "html": msg.HTMLBody}
	if msg.ReplyTo != "" {
		payload["reply_to"] = map[string]string{"email": msg.ReplyTo}
	}
	return emailkit.SendJSON(ctx, p.httpClient, p.cfg.OperationTimeout, strings.TrimRight(p.cfg.BaseURL, "/")+"/v1/email", payload, map[string]string{"Authorization": "Bearer " + p.cfg.Token})
}
func (p *Provider) Close() error { return nil }
