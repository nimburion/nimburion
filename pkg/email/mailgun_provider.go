package email

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// MailgunConfig configures Mailgun adapter.
type MailgunConfig struct {
	APIKey           string
	Domain           string
	From             string
	BaseURL          string
	OperationTimeout time.Duration
	HTTPClient       *http.Client
}

// MailgunProvider sends email through Mailgun API.
type MailgunProvider struct {
	cfg        MailgunConfig
	httpClient *http.Client
	log        logger.Logger
}

// NewMailgunProvider creates a Mailgun adapter.
func NewMailgunProvider(cfg MailgunConfig, log logger.Logger) (*MailgunProvider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("mailgun api key is required")
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
	return &MailgunProvider{
		cfg:        cfg,
		httpClient: defaultHTTPClient(cfg.HTTPClient, cfg.OperationTimeout),
		log:        log,
	}, nil
}

// Send sends email via Mailgun.
func (p *MailgunProvider) Send(ctx context.Context, message Message) error {
	msg := message.normalized()
	msg, err := applyDefaultSender(msg, p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.validate(); err != nil {
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

	cctx, cancel := withTimeout(ctx, p.cfg.OperationTimeout)
	defer cancel()

	endpoint := strings.TrimRight(p.cfg.BaseURL, "/") + "/v3/" + p.cfg.Domain + "/messages"
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth("api", p.cfg.APIKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mailgun send failed with status %d", resp.StatusCode)
	}
	return nil
}

// Close releases resources.
func (p *MailgunProvider) Close() error {
	return nil
}
