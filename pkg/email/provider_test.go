package email_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/email"
	"github.com/nimburion/nimburion/pkg/email/brevo"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
	"github.com/nimburion/nimburion/pkg/email/construct"
	"github.com/nimburion/nimburion/pkg/email/mailchimp"
	"github.com/nimburion/nimburion/pkg/email/mailersend"
	"github.com/nimburion/nimburion/pkg/email/mailgun"
	"github.com/nimburion/nimburion/pkg/email/mailjet"
	"github.com/nimburion/nimburion/pkg/email/mailtrap"
	"github.com/nimburion/nimburion/pkg/email/postmark"
	"github.com/nimburion/nimburion/pkg/email/sendgrid"
	"github.com/nimburion/nimburion/pkg/email/sendpulse"
	"github.com/nimburion/nimburion/pkg/email/ses"
	"github.com/nimburion/nimburion/pkg/email/smtp"
	"github.com/nimburion/nimburion/pkg/email/smtp2go"
)

func TestConstructNewProvider(t *testing.T) {
	provider, err := construct.NewProvider(emailconfig.Config{
		Provider: "smtp",
		SMTP: emailconfig.SMTPConfig{
			Host: "smtp.example.com",
			Port: 587,
			From: "noreply@example.com",
		},
	}, nil)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if provider == nil {
		t.Fatal("expected provider")
	}
}

func TestConstructNewProvider_UnsupportedProviderIsTyped(t *testing.T) {
	_, err := construct.NewProvider(emailconfig.Config{Provider: "unknown"}, nil)
	if err == nil {
		t.Fatal("expected unsupported provider error")
	}
	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != "validation.email.provider.unsupported" {
		t.Fatalf("Code = %q", appErr.Code)
	}
}

func TestProviderConstructors_ValidationErrorsAreTyped(t *testing.T) {
	tests := []struct {
		name     string
		buildErr func() error
		wantCode string
	}{
		{name: "sendgrid", buildErr: func() error { _, err := sendgrid.New(emailconfig.TokenConfig{}, nil); return err }, wantCode: "validation.email.sendgrid.token.required"},
		{name: "mailgun token", buildErr: func() error { _, err := mailgun.New(emailconfig.MailgunConfig{}, nil); return err }, wantCode: "validation.email.mailgun.token.required"},
		{name: "mailgun domain", buildErr: func() error { _, err := mailgun.New(emailconfig.MailgunConfig{Token: "x"}, nil); return err }, wantCode: "validation.email.mailgun.domain.required"},
		{name: "smtp", buildErr: func() error { _, err := smtp.New(emailconfig.SMTPConfig{}, nil); return err }, wantCode: "validation.email.smtp.host.required"},
		{name: "brevo", buildErr: func() error { _, err := brevo.New(emailconfig.TokenConfig{}, nil); return err }, wantCode: "validation.email.brevo.token.required"},
		{name: "mailchimp", buildErr: func() error { _, err := mailchimp.New(emailconfig.TokenConfig{}, nil); return err }, wantCode: "validation.email.mailchimp.token.required"},
		{name: "mailersend", buildErr: func() error { _, err := mailersend.New(emailconfig.TokenConfig{}, nil); return err }, wantCode: "validation.email.mailersend.token.required"},
		{name: "mailjet key", buildErr: func() error { _, err := mailjet.New(emailconfig.MailjetConfig{}, nil); return err }, wantCode: "validation.email.mailjet.api_key.required"},
		{name: "mailjet secret", buildErr: func() error { _, err := mailjet.New(emailconfig.MailjetConfig{APIKey: "k"}, nil); return err }, wantCode: "validation.email.mailjet.api_secret.required"},
		{name: "mailtrap", buildErr: func() error { _, err := mailtrap.New(emailconfig.TokenConfig{}, nil); return err }, wantCode: "validation.email.mailtrap.token.required"},
		{name: "postmark", buildErr: func() error { _, err := postmark.New(emailconfig.PostmarkConfig{}, nil); return err }, wantCode: "validation.email.postmark.server_token.required"},
		{name: "sendpulse", buildErr: func() error { _, err := sendpulse.New(emailconfig.TokenConfig{}, nil); return err }, wantCode: "validation.email.sendpulse.token.required"},
		{name: "smtp2go", buildErr: func() error { _, err := smtp2go.New(emailconfig.TokenConfig{}, nil); return err }, wantCode: "validation.email.smtp2go.token.required"},
		{name: "ses", buildErr: func() error { _, err := ses.New(emailconfig.SESConfig{}, nil); return err }, wantCode: "validation.email.ses.region.required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.buildErr()
			if err == nil {
				t.Fatal("expected validation error")
			}
			var appErr *coreerrors.AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("expected AppError, got %T", err)
			}
			if appErr.Code != tt.wantCode {
				t.Fatalf("Code = %q", appErr.Code)
			}
		})
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
	p, err := sendgrid.New(emailconfig.TokenConfig{Token: "sg-key", From: "noreply@example.com", BaseURL: server.URL, OperationTimeout: 2 * time.Second}, nil)
	if err != nil {
		t.Fatalf("new sendgrid: %v", err)
	}
	if err := p.Send(context.Background(), email.Message{To: []string{"u@example.com"}, Subject: "hello", TextBody: "body"}); err != nil {
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
	p, err := mailgun.New(emailconfig.MailgunConfig{Token: "mg-key", Domain: "mg.example.com", From: "noreply@example.com", BaseURL: server.URL, OperationTimeout: 2 * time.Second}, nil)
	if err != nil {
		t.Fatalf("new mailgun: %v", err)
	}
	if err := p.Send(context.Background(), email.Message{To: []string{"u@example.com"}, Subject: "hello", TextBody: "body"}); err != nil {
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
	p, err := mailchimp.New(emailconfig.TokenConfig{Token: "mc-key", From: "noreply@example.com", BaseURL: server.URL, OperationTimeout: 2 * time.Second}, nil)
	if err != nil {
		t.Fatalf("new mailchimp: %v", err)
	}
	if err := p.Send(context.Background(), email.Message{To: []string{"u@example.com"}, Subject: "hello", HTMLBody: "<p>body</p>"}); err != nil {
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
	p, err := ses.New(emailconfig.SESConfig{Region: "eu-west-1", From: "noreply@example.com", Endpoint: server.URL, AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: "secret", OperationTimeout: 2 * time.Second}, nil)
	if err != nil {
		t.Fatalf("new ses: %v", err)
	}
	if err := p.Send(context.Background(), email.Message{To: []string{"u@example.com"}, Subject: "hello", TextBody: "body"}); err != nil {
		t.Fatalf("send ses: %v", err)
	}
}

func TestAdditionalProviders_SendAndClose(t *testing.T) {
	cases := []struct {
		name string
		new  func(url string) (email.Provider, error)
	}{
		{"mailersend", func(url string) (email.Provider, error) {
			return mailersend.New(emailconfig.TokenConfig{Token: "mls-key", From: "noreply@example.com", BaseURL: url}, nil)
		}},
		{"postmark", func(url string) (email.Provider, error) {
			return postmark.New(emailconfig.PostmarkConfig{ServerToken: "pm-key", From: "noreply@example.com", BaseURL: url}, nil)
		}},
		{"mailtrap", func(url string) (email.Provider, error) {
			return mailtrap.New(emailconfig.TokenConfig{Token: "mt-key", From: "noreply@example.com", BaseURL: url}, nil)
		}},
		{"smtp2go", func(url string) (email.Provider, error) {
			return smtp2go.New(emailconfig.TokenConfig{Token: "s2g-key", From: "noreply@example.com", BaseURL: url}, nil)
		}},
		{"sendpulse", func(url string) (email.Provider, error) {
			return sendpulse.New(emailconfig.TokenConfig{Token: "sp-key", From: "noreply@example.com", BaseURL: url}, nil)
		}},
		{"brevo", func(url string) (email.Provider, error) {
			return brevo.New(emailconfig.TokenConfig{Token: "br-key", From: "noreply@example.com", BaseURL: url}, nil)
		}},
		{"mailjet", func(url string) (email.Provider, error) {
			return mailjet.New(emailconfig.MailjetConfig{APIKey: "mj-key", APISecret: "mj-secret", From: "noreply@example.com", BaseURL: url}, nil)
		}},
		{"smtp", func(_ string) (email.Provider, error) {
			return smtp.New(emailconfig.SMTPConfig{Host: "smtp.example.com", Port: 587, From: "noreply@example.com"}, nil)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
			defer server.Close()
			p, err := tc.new(server.URL)
			if err != nil {
				t.Fatalf("new provider: %v", err)
			}
			if err := p.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}
		})
	}
}
