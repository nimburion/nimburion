package mailchimp

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

type Config = emailconfig.TokenConfig

type Provider struct {
	cfg        Config
	httpClient *http.Client
	log        logger.Logger
}

func New(cfg Config, log logger.Logger) (*Provider, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, coreerrors.NewValidationWithCode("validation.email.mailchimp.token.required", "mailchimp token is required", nil, nil)
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://mandrillapp.com/api/1.0"
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
	payload := map[string]interface{}{
		"key": p.cfg.Token,
		"message": map[string]interface{}{
			"from_email": msg.From, "subject": msg.Subject, "text": msg.TextBody, "html": msg.HTMLBody,
			"to": emailkit.MapMailchimpRecipients(msg.To, msg.Cc, msg.Bcc), "headers": emailkit.CloneStringMap(msg.Headers),
		},
	}
	if msg.ReplyTo != "" {
		messagePayload, ok := payload["message"].(map[string]interface{})
		if !ok {
			return coreerrors.NewInternal(fmt.Sprintf("mailchimp payload message has unexpected type %T", payload["message"]), nil).
				WithDetails(map[string]interface{}{"provider": "mailchimp"})
		}
		headers, ok := messagePayload["headers"].(map[string]string)
		if !ok {
			return coreerrors.NewInternal(fmt.Sprintf("mailchimp payload headers has unexpected type %T", messagePayload["headers"]), nil).
				WithDetails(map[string]interface{}{"provider": "mailchimp"})
		}
		headers["Reply-To"] = msg.ReplyTo
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	cctx, cancel := emailkit.WithTimeout(ctx, p.cfg.OperationTimeout)
	defer cancel()
	endpoint := strings.TrimRight(p.cfg.BaseURL, "/") + "/messages/send.json"
	if err := emailkit.ValidateEndpointURL(endpoint); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { emailkit.IgnoreCloseError(resp.Body.Close()) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return coreerrors.NewUnavailable(fmt.Sprintf("mailchimp send failed with status %d", resp.StatusCode), nil).
			WithDetails(map[string]interface{}{"provider": "mailchimp", "status_code": resp.StatusCode})
	}
	return nil
}

func (p *Provider) Close() error { return nil }
