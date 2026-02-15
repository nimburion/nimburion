package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// SendGridConfig configures SendGrid adapter.
type SendGridConfig struct {
	APIKey           string
	From             string
	BaseURL          string
	OperationTimeout time.Duration
	HTTPClient       *http.Client
}

// SendGridProvider sends email through SendGrid API.
type SendGridProvider struct {
	cfg        SendGridConfig
	httpClient *http.Client
	log        logger.Logger
}

// NewSendGridProvider creates a SendGrid adapter.
func NewSendGridProvider(cfg SendGridConfig, log logger.Logger) (*SendGridProvider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("sendgrid api key is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.sendgrid.com"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &SendGridProvider{
		cfg:        cfg,
		httpClient: defaultHTTPClient(cfg.HTTPClient, cfg.OperationTimeout),
		log:        log,
	}, nil
}

// Send sends email via SendGrid.
func (p *SendGridProvider) Send(ctx context.Context, message Message) error {
	msg := message.normalized()
	msg, err := applyDefaultSender(msg, p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.validate(); err != nil {
		return err
	}

	payload := map[string]interface{}{
		"personalizations": []map[string]interface{}{
			{
				"to":  mapRecipients(msg.To),
				"cc":  mapRecipients(msg.Cc),
				"bcc": mapRecipients(msg.Bcc),
			},
		},
		"from":    map[string]string{"email": msg.From},
		"subject": msg.Subject,
		"content": mapContent(msg),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	cctx, cancel := withTimeout(ctx, p.cfg.OperationTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(cctx, http.MethodPost, strings.TrimRight(p.cfg.BaseURL, "/")+"/v3/mail/send", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sendgrid send failed with status %d", resp.StatusCode)
	}
	return nil
}

// Close releases resources.
func (p *SendGridProvider) Close() error {
	return nil
}

func mapRecipients(emails []string) []map[string]string {
	out := make([]map[string]string, 0, len(emails))
	for _, email := range emails {
		if strings.TrimSpace(email) == "" {
			continue
		}
		out = append(out, map[string]string{"email": email})
	}
	return out
}

func mapContent(msg Message) []map[string]string {
	content := make([]map[string]string, 0, 2)
	if strings.TrimSpace(msg.TextBody) != "" {
		content = append(content, map[string]string{
			"type":  "text/plain",
			"value": msg.TextBody,
		})
	}
	if strings.TrimSpace(msg.HTMLBody) != "" {
		content = append(content, map[string]string{
			"type":  "text/html",
			"value": msg.HTMLBody,
		})
	}
	return content
}
