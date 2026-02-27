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

// MailchimpConfig configures Mailchimp Transactional (Mandrill) adapter.
type MailchimpConfig struct {
	APIKey           string
	From             string
	BaseURL          string
	OperationTimeout time.Duration
	HTTPClient       *http.Client
}

// MailchimpProvider sends email through Mailchimp Transactional API.
type MailchimpProvider struct {
	cfg        MailchimpConfig
	httpClient *http.Client
	log        logger.Logger
}

// NewMailchimpProvider creates a Mailchimp adapter.
func NewMailchimpProvider(cfg MailchimpConfig, log logger.Logger) (*MailchimpProvider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("mailchimp api key is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://mandrillapp.com/api/1.0"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &MailchimpProvider{
		cfg:        cfg,
		httpClient: defaultHTTPClient(cfg.HTTPClient, cfg.OperationTimeout),
		log:        log,
	}, nil
}

// Send sends email via Mailchimp Transactional (Mandrill).
func (p *MailchimpProvider) Send(ctx context.Context, message Message) error {
	msg := message.normalized()
	msg, err := applyDefaultSender(msg, p.cfg.From)
	if err != nil {
		return err
	}
	if validateErr := msg.validate(); validateErr != nil {
		return validateErr
	}

	payload := map[string]interface{}{
		"key": p.cfg.APIKey,
		"message": map[string]interface{}{
			"from_email": msg.From,
			"subject":    msg.Subject,
			"text":       msg.TextBody,
			"html":       msg.HTMLBody,
			"to":         mapMailchimpRecipients(msg.To, msg.Cc, msg.Bcc),
			"headers":    cloneStringMap(msg.Headers),
		},
	}
	if msg.ReplyTo != "" {
		headers := payload["message"].(map[string]interface{})["headers"].(map[string]string)
		headers["Reply-To"] = msg.ReplyTo
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	cctx, cancel := withTimeout(ctx, p.cfg.OperationTimeout)
	defer cancel()

	endpoint := strings.TrimRight(p.cfg.BaseURL, "/") + "/messages/send.json"
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mailchimp send failed with status %d", resp.StatusCode)
	}
	return nil
}

func mapMailchimpRecipients(to, cc, bcc []string) []map[string]string {
	out := make([]map[string]string, 0, len(to)+len(cc)+len(bcc))
	for _, email := range to {
		out = append(out, map[string]string{"email": email, "type": "to"})
	}
	for _, email := range cc {
		out = append(out, map[string]string{"email": email, "type": "cc"})
	}
	for _, email := range bcc {
		out = append(out, map[string]string{"email": email, "type": "bcc"})
	}
	return out
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out
}

// Close releases resources.
func (p *MailchimpProvider) Close() error {
	return nil
}
