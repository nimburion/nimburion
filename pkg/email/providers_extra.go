package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type MailerSendConfig struct {
	APIKey           string
	From             string
	BaseURL          string
	OperationTimeout time.Duration
	HTTPClient       *http.Client
}

type MailerSendProvider struct {
	cfg        MailerSendConfig
	httpClient *http.Client
	log        logger.Logger
}

func NewMailerSendProvider(cfg MailerSendConfig, log logger.Logger) (*MailerSendProvider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("mailersend api key is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.mailersend.com"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &MailerSendProvider{cfg: cfg, httpClient: defaultHTTPClient(cfg.HTTPClient, cfg.OperationTimeout), log: log}, nil
}

func (p *MailerSendProvider) Send(ctx context.Context, message Message) error {
	msg, err := applyDefaultSender(message.normalized(), p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.validate(); err != nil {
		return err
	}
	payload := map[string]interface{}{
		"from":    map[string]string{"email": msg.From},
		"to":      mapRecipients(msg.To),
		"cc":      mapRecipients(msg.Cc),
		"bcc":     mapRecipients(msg.Bcc),
		"subject": msg.Subject,
		"text":    msg.TextBody,
		"html":    msg.HTMLBody,
	}
	if msg.ReplyTo != "" {
		payload["reply_to"] = map[string]string{"email": msg.ReplyTo}
	}
	return sendJSONWithAuth(ctx, p.httpClient, p.cfg.OperationTimeout, http.MethodPost, strings.TrimRight(p.cfg.BaseURL, "/")+"/v1/email", payload, map[string]string{
		"Authorization": "Bearer " + p.cfg.APIKey,
	})
}

func (p *MailerSendProvider) Close() error { return nil }

type PostmarkConfig struct {
	ServerToken      string
	From             string
	BaseURL          string
	OperationTimeout time.Duration
	HTTPClient       *http.Client
}

type PostmarkProvider struct {
	cfg        PostmarkConfig
	httpClient *http.Client
	log        logger.Logger
}

func NewPostmarkProvider(cfg PostmarkConfig, log logger.Logger) (*PostmarkProvider, error) {
	if strings.TrimSpace(cfg.ServerToken) == "" {
		return nil, fmt.Errorf("postmark server token is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.postmarkapp.com"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &PostmarkProvider{cfg: cfg, httpClient: defaultHTTPClient(cfg.HTTPClient, cfg.OperationTimeout), log: log}, nil
}

func (p *PostmarkProvider) Send(ctx context.Context, message Message) error {
	msg, err := applyDefaultSender(message.normalized(), p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.validate(); err != nil {
		return err
	}
	payload := map[string]interface{}{
		"From":     msg.From,
		"To":       strings.Join(msg.To, ","),
		"Cc":       strings.Join(msg.Cc, ","),
		"Bcc":      strings.Join(msg.Bcc, ","),
		"Subject":  msg.Subject,
		"TextBody": msg.TextBody,
		"HtmlBody": msg.HTMLBody,
		"ReplyTo":  msg.ReplyTo,
	}
	return sendJSONWithAuth(ctx, p.httpClient, p.cfg.OperationTimeout, http.MethodPost, strings.TrimRight(p.cfg.BaseURL, "/")+"/email", payload, map[string]string{
		"X-Postmark-Server-Token": p.cfg.ServerToken,
	})
}

func (p *PostmarkProvider) Close() error { return nil }

type MailtrapConfig struct {
	Token            string
	From             string
	BaseURL          string
	OperationTimeout time.Duration
	HTTPClient       *http.Client
}

type MailtrapProvider struct {
	cfg        MailtrapConfig
	httpClient *http.Client
	log        logger.Logger
}

func NewMailtrapProvider(cfg MailtrapConfig, log logger.Logger) (*MailtrapProvider, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, fmt.Errorf("mailtrap token is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://send.api.mailtrap.io"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &MailtrapProvider{cfg: cfg, httpClient: defaultHTTPClient(cfg.HTTPClient, cfg.OperationTimeout), log: log}, nil
}

func (p *MailtrapProvider) Send(ctx context.Context, message Message) error {
	msg, err := applyDefaultSender(message.normalized(), p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.validate(); err != nil {
		return err
	}
	payload := map[string]interface{}{
		"from":    map[string]string{"email": msg.From},
		"to":      mapRecipients(msg.To),
		"cc":      mapRecipients(msg.Cc),
		"bcc":     mapRecipients(msg.Bcc),
		"subject": msg.Subject,
		"text":    msg.TextBody,
		"html":    msg.HTMLBody,
	}
	return sendJSONWithAuth(ctx, p.httpClient, p.cfg.OperationTimeout, http.MethodPost, strings.TrimRight(p.cfg.BaseURL, "/")+"/api/send", payload, map[string]string{
		"Authorization": "Bearer " + p.cfg.Token,
	})
}

func (p *MailtrapProvider) Close() error { return nil }

type SMTP2GOConfig struct {
	APIKey           string
	From             string
	BaseURL          string
	OperationTimeout time.Duration
	HTTPClient       *http.Client
}

type SMTP2GOProvider struct {
	cfg        SMTP2GOConfig
	httpClient *http.Client
	log        logger.Logger
}

func NewSMTP2GOProvider(cfg SMTP2GOConfig, log logger.Logger) (*SMTP2GOProvider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("smtp2go api key is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.smtp2go.com"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &SMTP2GOProvider{cfg: cfg, httpClient: defaultHTTPClient(cfg.HTTPClient, cfg.OperationTimeout), log: log}, nil
}

func (p *SMTP2GOProvider) Send(ctx context.Context, message Message) error {
	msg, err := applyDefaultSender(message.normalized(), p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.validate(); err != nil {
		return err
	}
	payload := map[string]interface{}{
		"api_key":   p.cfg.APIKey,
		"sender":    msg.From,
		"to":        msg.To,
		"cc":        msg.Cc,
		"bcc":       msg.Bcc,
		"subject":   msg.Subject,
		"text_body": msg.TextBody,
		"html_body": msg.HTMLBody,
	}
	return sendJSONWithAuth(ctx, p.httpClient, p.cfg.OperationTimeout, http.MethodPost, strings.TrimRight(p.cfg.BaseURL, "/")+"/v3/email/send", payload, nil)
}

func (p *SMTP2GOProvider) Close() error { return nil }

type SendPulseConfig struct {
	Token            string
	From             string
	BaseURL          string
	OperationTimeout time.Duration
	HTTPClient       *http.Client
}

type SendPulseProvider struct {
	cfg        SendPulseConfig
	httpClient *http.Client
	log        logger.Logger
}

func NewSendPulseProvider(cfg SendPulseConfig, log logger.Logger) (*SendPulseProvider, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, fmt.Errorf("sendpulse token is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.sendpulse.com"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &SendPulseProvider{cfg: cfg, httpClient: defaultHTTPClient(cfg.HTTPClient, cfg.OperationTimeout), log: log}, nil
}

func (p *SendPulseProvider) Send(ctx context.Context, message Message) error {
	msg, err := applyDefaultSender(message.normalized(), p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.validate(); err != nil {
		return err
	}
	payload := map[string]interface{}{
		"email": map[string]interface{}{
			"subject": msg.Subject,
			"from":    map[string]string{"email": msg.From},
			"to":      mapRecipients(msg.To),
			"cc":      mapRecipients(msg.Cc),
			"bcc":     mapRecipients(msg.Bcc),
			"text":    msg.TextBody,
			"html":    msg.HTMLBody,
		},
	}
	return sendJSONWithAuth(ctx, p.httpClient, p.cfg.OperationTimeout, http.MethodPost, strings.TrimRight(p.cfg.BaseURL, "/")+"/smtp/emails", payload, map[string]string{
		"Authorization": "Bearer " + p.cfg.Token,
	})
}

func (p *SendPulseProvider) Close() error { return nil }

type BrevoConfig struct {
	APIKey           string
	From             string
	BaseURL          string
	OperationTimeout time.Duration
	HTTPClient       *http.Client
}

type BrevoProvider struct {
	cfg        BrevoConfig
	httpClient *http.Client
	log        logger.Logger
}

func NewBrevoProvider(cfg BrevoConfig, log logger.Logger) (*BrevoProvider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("brevo api key is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.brevo.com"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &BrevoProvider{cfg: cfg, httpClient: defaultHTTPClient(cfg.HTTPClient, cfg.OperationTimeout), log: log}, nil
}

func (p *BrevoProvider) Send(ctx context.Context, message Message) error {
	msg, err := applyDefaultSender(message.normalized(), p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.validate(); err != nil {
		return err
	}
	payload := map[string]interface{}{
		"sender":      map[string]string{"email": msg.From},
		"to":          mapRecipients(msg.To),
		"cc":          mapRecipients(msg.Cc),
		"bcc":         mapRecipients(msg.Bcc),
		"subject":     msg.Subject,
		"textContent": msg.TextBody,
		"htmlContent": msg.HTMLBody,
	}
	return sendJSONWithAuth(ctx, p.httpClient, p.cfg.OperationTimeout, http.MethodPost, strings.TrimRight(p.cfg.BaseURL, "/")+"/v3/smtp/email", payload, map[string]string{
		"api-key": p.cfg.APIKey,
	})
}

func (p *BrevoProvider) Close() error { return nil }

type MailjetConfig struct {
	APIKey           string
	APISecret        string
	From             string
	BaseURL          string
	OperationTimeout time.Duration
	HTTPClient       *http.Client
}

type MailjetProvider struct {
	cfg        MailjetConfig
	httpClient *http.Client
	log        logger.Logger
}

func NewMailjetProvider(cfg MailjetConfig, log logger.Logger) (*MailjetProvider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("mailjet api key is required")
	}
	if strings.TrimSpace(cfg.APISecret) == "" {
		return nil, fmt.Errorf("mailjet api secret is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.mailjet.com"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &MailjetProvider{cfg: cfg, httpClient: defaultHTTPClient(cfg.HTTPClient, cfg.OperationTimeout), log: log}, nil
}

func (p *MailjetProvider) Send(ctx context.Context, message Message) error {
	msg, err := applyDefaultSender(message.normalized(), p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.validate(); err != nil {
		return err
	}
	payload := map[string]interface{}{
		"Messages": []map[string]interface{}{
			{
				"From":     map[string]string{"Email": msg.From},
				"To":       mapRecipientsWithKey(msg.To, "Email"),
				"Cc":       mapRecipientsWithKey(msg.Cc, "Email"),
				"Bcc":      mapRecipientsWithKey(msg.Bcc, "Email"),
				"Subject":  msg.Subject,
				"TextPart": msg.TextBody,
				"HTMLPart": msg.HTMLBody,
			},
		},
	}
	auth := base64.StdEncoding.EncodeToString([]byte(p.cfg.APIKey + ":" + p.cfg.APISecret))
	return sendJSONWithAuth(ctx, p.httpClient, p.cfg.OperationTimeout, http.MethodPost, strings.TrimRight(p.cfg.BaseURL, "/")+"/v3.1/send", payload, map[string]string{
		"Authorization": "Basic " + auth,
	})
}

func (p *MailjetProvider) Close() error { return nil }

func sendJSONWithAuth(ctx context.Context, client *http.Client, timeout time.Duration, method, endpoint string, payload interface{}, headers map[string]string) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	cctx, cancel := withTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, method, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("email send failed with status %d", resp.StatusCode)
	}
	return nil
}

func mapRecipientsWithKey(emails []string, key string) []map[string]string {
	out := make([]map[string]string, 0, len(emails))
	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		out = append(out, map[string]string{key: email})
	}
	return out
}
