package mailgun

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nimburion/nimburion/internal/emailkit"
	"github.com/nimburion/nimburion/pkg/email"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type Config = emailconfig.MailgunConfig

type Provider struct {
	cfg        Config
	httpClient *http.Client
	log        logger.Logger
}

func New(cfg Config, log logger.Logger) (*Provider, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, fmt.Errorf("mailgun token is required")
	}
	if strings.TrimSpace(cfg.Domain) == "" {
		return nil, fmt.Errorf("mailgun domain is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.mailgun.net"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &Provider{cfg: cfg, httpClient: emailkit.DefaultHTTPClient(nil, cfg.OperationTimeout), log: log}, nil
}

func (p *Provider) Send(ctx context.Context, message email.Message) error {
	msg := message.Normalized()
	msg, err := email.ApplyDefaultSender(msg, p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.Validate(); err != nil {
		return err
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
	if err := emailkit.ValidateEndpointURL(endpoint); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth("api", p.cfg.Token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { emailkit.IgnoreCloseError(resp.Body.Close()) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mailgun send failed with status %d", resp.StatusCode)
	}
	return nil
}

func (p *Provider) Close() error { return nil }
