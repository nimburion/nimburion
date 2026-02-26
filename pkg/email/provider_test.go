package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func TestFactory_NewProvider(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: ProviderSMTP,
		SMTP: SMTPConfig{
			Host: "smtp.example.com",
			Port: 587,
			From: "noreply@example.com",
		},
	}, nil)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if provider == nil {
		t.Fatalf("expected provider instance")
	}
}

func TestFactory_NewProvider_ExtraProviders(t *testing.T) {
	cases := []Config{
		{Provider: ProviderMailerSend, MailerSend: MailerSendConfig{APIKey: "k", From: "noreply@example.com"}},
		{Provider: ProviderPostmark, Postmark: PostmarkConfig{ServerToken: "k", From: "noreply@example.com"}},
		{Provider: ProviderMailtrap, Mailtrap: MailtrapConfig{Token: "k", From: "noreply@example.com"}},
		{Provider: ProviderSMTP2GO, SMTP2GO: SMTP2GOConfig{APIKey: "k", From: "noreply@example.com"}},
		{Provider: ProviderSendPulse, SendPulse: SendPulseConfig{Token: "k", From: "noreply@example.com"}},
		{Provider: ProviderBrevo, Brevo: BrevoConfig{APIKey: "k", From: "noreply@example.com"}},
		{Provider: ProviderMailjet, Mailjet: MailjetConfig{APIKey: "k", APISecret: "s", From: "noreply@example.com"}},
	}
	for _, cfg := range cases {
		p, err := NewProvider(cfg, nil)
		if err != nil {
			t.Fatalf("new provider for %s: %v", cfg.Provider, err)
		}
		if p == nil {
			t.Fatalf("expected provider for %s", cfg.Provider)
		}
	}
}

func TestSMTPProvider_Send(t *testing.T) {
	p, err := NewSMTPProvider(SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
		From: "noreply@example.com",
	}, nil)
	if err != nil {
		t.Fatalf("new smtp provider: %v", err)
	}

	called := false
	p.sendMail = func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
		called = true
		if addr != "smtp.example.com:587" {
			t.Fatalf("unexpected smtp addr: %s", addr)
		}
		if from != "noreply@example.com" {
			t.Fatalf("unexpected from: %s", from)
		}
		if len(to) != 1 || to[0] != "user@example.com" {
			t.Fatalf("unexpected recipients: %v", to)
		}
		if !strings.Contains(string(msg), "Subject: hello") {
			t.Fatalf("expected subject in mime")
		}
		return nil
	}

	if err := p.Send(context.Background(), Message{
		To:       []string{"user@example.com"},
		Subject:  "hello",
		TextBody: "body",
	}); err != nil {
		t.Fatalf("send smtp: %v", err)
	}
	if !called {
		t.Fatalf("expected smtp send call")
	}
}

func TestSendGridProvider_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sg-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["subject"] != "hello" {
			t.Fatalf("unexpected subject payload")
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	p, err := NewSendGridProvider(SendGridConfig{
		APIKey:           "sg-key",
		From:             "noreply@example.com",
		BaseURL:          server.URL,
		OperationTimeout: 2 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("new sendgrid: %v", err)
	}
	if err := p.Send(context.Background(), Message{
		To:       []string{"u@example.com"},
		Subject:  "hello",
		TextBody: "body",
	}); err != nil {
		t.Fatalf("send sendgrid: %v", err)
	}
}

func TestMailgunProvider_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "api" || pass != "mg-key" {
			t.Fatalf("unexpected basic auth")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.Form.Get("subject") != "hello" {
			t.Fatalf("expected subject")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p, err := NewMailgunProvider(MailgunConfig{
		APIKey:           "mg-key",
		Domain:           "mg.example.com",
		From:             "noreply@example.com",
		BaseURL:          server.URL,
		OperationTimeout: 2 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("new mailgun: %v", err)
	}
	if err := p.Send(context.Background(), Message{
		To:       []string{"u@example.com"},
		Subject:  "hello",
		TextBody: "body",
	}); err != nil {
		t.Fatalf("send mailgun: %v", err)
	}
}

func TestMailchimpProvider_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["key"] != "mc-key" {
			t.Fatalf("unexpected api key")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p, err := NewMailchimpProvider(MailchimpConfig{
		APIKey:           "mc-key",
		From:             "noreply@example.com",
		BaseURL:          server.URL,
		OperationTimeout: 2 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("new mailchimp: %v", err)
	}
	if err := p.Send(context.Background(), Message{
		To:       []string{"u@example.com"},
		Subject:  "hello",
		HTMLBody: "<p>body</p>",
	}); err != nil {
		t.Fatalf("send mailchimp: %v", err)
	}
}

func TestSESProvider_SendSignedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); !strings.Contains(got, "AWS4-HMAC-SHA256") {
			t.Fatalf("expected aws sigv4 authorization header")
		}
		if got := r.Header.Get("X-Amz-Date"); got == "" {
			t.Fatalf("expected x-amz-date")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p, err := NewSESProvider(SESConfig{
		Region:           "eu-west-1",
		From:             "noreply@example.com",
		Endpoint:         server.URL,
		AccessKeyID:      "AKIDEXAMPLE",
		SecretAccessKey:  "secret",
		OperationTimeout: 2 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("new ses: %v", err)
	}

	if err := p.Send(context.Background(), Message{
		To:       []string{"u@example.com"},
		Subject:  "hello",
		TextBody: "body",
	}); err != nil {
		t.Fatalf("send ses: %v", err)
	}
}

func TestMailerSendProvider_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer mls-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	p, err := NewMailerSendProvider(MailerSendConfig{APIKey: "mls-key", From: "noreply@example.com", BaseURL: server.URL}, nil)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if err := p.Send(context.Background(), Message{To: []string{"u@example.com"}, Subject: "s", TextBody: "b"}); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func TestPostmarkProvider_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Postmark-Server-Token"); got != "pm-key" {
			t.Fatalf("unexpected token: %s", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p, err := NewPostmarkProvider(PostmarkConfig{ServerToken: "pm-key", From: "noreply@example.com", BaseURL: server.URL}, nil)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if err := p.Send(context.Background(), Message{To: []string{"u@example.com"}, Subject: "s", TextBody: "b"}); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func TestMailtrapProvider_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer mt-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p, err := NewMailtrapProvider(MailtrapConfig{Token: "mt-key", From: "noreply@example.com", BaseURL: server.URL}, nil)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if err := p.Send(context.Background(), Message{To: []string{"u@example.com"}, Subject: "s", TextBody: "b"}); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func TestSMTP2GOProvider_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["api_key"] != "s2g-key" {
			t.Fatalf("expected api_key in payload")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p, err := NewSMTP2GOProvider(SMTP2GOConfig{APIKey: "s2g-key", From: "noreply@example.com", BaseURL: server.URL}, nil)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if err := p.Send(context.Background(), Message{To: []string{"u@example.com"}, Subject: "s", TextBody: "b"}); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func TestSendPulseProvider_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sp-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p, err := NewSendPulseProvider(SendPulseConfig{Token: "sp-key", From: "noreply@example.com", BaseURL: server.URL}, nil)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if err := p.Send(context.Background(), Message{To: []string{"u@example.com"}, Subject: "s", TextBody: "b"}); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func TestBrevoProvider_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("api-key"); got != "br-key" {
			t.Fatalf("unexpected api-key header: %s", got)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	p, err := NewBrevoProvider(BrevoConfig{APIKey: "br-key", From: "noreply@example.com", BaseURL: server.URL}, nil)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if err := p.Send(context.Background(), Message{To: []string{"u@example.com"}, Subject: "s", TextBody: "b"}); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func TestMailjetProvider_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Basic ") {
			t.Fatalf("expected basic auth header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p, err := NewMailjetProvider(MailjetConfig{APIKey: "mj-key", APISecret: "mj-secret", From: "noreply@example.com", BaseURL: server.URL}, nil)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if err := p.Send(context.Background(), Message{To: []string{"u@example.com"}, Subject: "s", TextBody: "b"}); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func TestProviderClose(t *testing.T) {
	providers := []struct {
		name string
		p    Provider
	}{
		{"mailchimp", &MailchimpProvider{}},
		{"mailgun", &MailgunProvider{}},
		{"mailersend", &MailerSendProvider{}},
		{"postmark", &PostmarkProvider{}},
		{"mailtrap", &MailtrapProvider{}},
		{"smtp2go", &SMTP2GOProvider{}},
		{"sendpulse", &SendPulseProvider{}},
		{"brevo", &BrevoProvider{}},
		{"mailjet", &MailjetProvider{}},
	}

	for _, tt := range providers {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.p.Close(); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
	}
}
