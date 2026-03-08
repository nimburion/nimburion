package emailkit

import (
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/email"
)

func TestBuildMIMEMessage_SanitizesHeaderValues(t *testing.T) {
	raw := BuildMIMEMessage(email.Message{
		From:    "sender@example.com\r\nBcc: injected@example.com",
		To:      []string{"to@example.com"},
		ReplyTo: "reply@example.com\r\nX-Injected: 1",
		Subject: "hello\r\nX-Injected: 1",
		Headers: map[string]string{
			"X-Test":          "ok\r\nX-Evil: 1",
			"Bad\r\nInjected": "value",
		},
		TextBody: "body",
	})

	rendered := string(raw)
	if strings.Contains(rendered, "\r\nBcc: injected@example.com\r\n") {
		t.Fatalf("unexpected injected Bcc header in MIME message: %q", rendered)
	}
	if strings.Contains(rendered, "\r\nX-Injected: 1\r\n") {
		t.Fatalf("unexpected injected header in MIME message: %q", rendered)
	}
	if strings.Contains(rendered, "Bad\r\nInjected") {
		t.Fatalf("unexpected unsanitized header name in MIME message: %q", rendered)
	}
	if !strings.Contains(rendered, "Subject: hello X-Injected: 1\r\n") {
		t.Fatalf("expected sanitized subject header, got %q", rendered)
	}
	if !strings.Contains(rendered, "X-Test: ok X-Evil: 1\r\n") {
		t.Fatalf("expected sanitized custom header, got %q", rendered)
	}
}
