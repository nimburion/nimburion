package email_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
		{"smtp", func(url string) (email.Provider, error) {
			return smtp.New(emailconfig.SMTPConfig{Host: "smtp.example.com", Port: 587, From: "noreply@example.com"}, nil)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
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
