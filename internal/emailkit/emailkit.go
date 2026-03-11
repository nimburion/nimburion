// Package emailkit provides shared helpers used by email provider implementations.
package emailkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/email"
)

// WithTimeout returns a derived context with the provided timeout when d is positive.
func WithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		// #nosec G118 -- the cancel function is returned to the caller, which owns cleanup.
		return context.WithCancel(ctx)
	}
	// #nosec G118 -- the cancel function is returned to the caller, which owns cleanup.
	return context.WithTimeout(ctx, d)
}

// DefaultHTTPClient returns client or a default client configured with timeout.
func DefaultHTTPClient(client *http.Client, timeout time.Duration) *http.Client {
	if client != nil {
		return client
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &http.Client{Timeout: timeout}
}

// ValidateEndpointURL validates that raw is an absolute HTTP or HTTPS endpoint URL.
func ValidateEndpointURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("endpoint must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("endpoint host is required")
	}
	return nil
}

// IgnoreCloseError intentionally ignores close errors in deferred cleanup paths.
func IgnoreCloseError(_ error) {}

// CloneStringMap returns a shallow copy of values.
func CloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out
}

// MapRecipients maps email addresses to provider payload objects with the email key.
func MapRecipients(emails []string) []map[string]string {
	out := make([]map[string]string, 0, len(emails))
	for _, addr := range emails {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		out = append(out, map[string]string{"email": addr})
	}
	return out
}

// MapRecipientsWithKey maps email addresses to provider payload objects using key.
func MapRecipientsWithKey(emails []string, key string) []map[string]string {
	out := make([]map[string]string, 0, len(emails))
	for _, addr := range emails {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		out = append(out, map[string]string{key: addr})
	}
	return out
}

// MapContent builds the SendGrid-style content payload for msg.
func MapContent(msg email.Message) []map[string]string {
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

// BuildMIMEMessage renders msg as a sanitized MIME email message.
func BuildMIMEMessage(msg email.Message) []byte {
	var b strings.Builder
	b.WriteString("From: " + sanitizeHeaderValue(msg.From) + "\r\n")
	if to := sanitizeAddressList(msg.To); len(to) > 0 {
		b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	}
	if cc := sanitizeAddressList(msg.Cc); len(cc) > 0 {
		b.WriteString("Cc: " + strings.Join(cc, ", ") + "\r\n")
	}
	if replyTo := sanitizeHeaderValue(msg.ReplyTo); replyTo != "" {
		b.WriteString("Reply-To: " + replyTo + "\r\n")
	}
	b.WriteString("Subject: " + sanitizeHeaderValue(msg.Subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	for k, v := range msg.Headers {
		key := sanitizeHeaderName(k)
		value := sanitizeHeaderValue(v)
		if key == "" || value == "" {
			continue
		}
		b.WriteString(key + ": " + value + "\r\n")
	}

	text := strings.TrimSpace(msg.TextBody)
	html := strings.TrimSpace(msg.HTMLBody)
	if text != "" && html != "" {
		boundary := "nimburion-alt-boundary"
		b.WriteString("Content-Type: multipart/alternative; boundary=" + boundary + "\r\n\r\n")
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		b.WriteString(text + "\r\n")
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		b.WriteString(html + "\r\n")
		b.WriteString("--" + boundary + "--\r\n")
		return []byte(b.String())
	}
	if html != "" {
		b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		b.WriteString(html)
		return []byte(b.String())
	}
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(text)
	return []byte(b.String())
}

func sanitizeHeaderName(value string) string {
	name := strings.TrimSpace(value)
	if name == "" || strings.ContainsAny(name, "\r\n:") {
		return ""
	}
	return name
}

func sanitizeHeaderValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "\r", " ")
	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	return strings.Join(strings.Fields(trimmed), " ")
}

func sanitizeAddressList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if sanitized := sanitizeHeaderValue(value); sanitized != "" {
			out = append(out, sanitized)
		}
	}
	return out
}

// MapMailchimpRecipients maps recipients to the Mailchimp recipient payload shape.
func MapMailchimpRecipients(to, cc, bcc []string) []map[string]string {
	out := make([]map[string]string, 0, len(to)+len(cc)+len(bcc))
	for _, addr := range to {
		out = append(out, map[string]string{"email": addr, "type": "to"})
	}
	for _, addr := range cc {
		out = append(out, map[string]string{"email": addr, "type": "cc"})
	}
	for _, addr := range bcc {
		out = append(out, map[string]string{"email": addr, "type": "bcc"})
	}
	return out
}

// SendJSON sends payload as JSON to endpoint using client and optional headers.
func SendJSON(ctx context.Context, client *http.Client, timeout time.Duration, endpoint string, payload interface{}, headers map[string]string) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if validationErr := ValidateEndpointURL(endpoint); validationErr != nil {
		return validationErr
	}
	cctx, cancel := WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodPost, endpoint, bytes.NewReader(raw))
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
	// #nosec G704 -- endpoint is validated with ValidateEndpointURL before issuing the request.
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { IgnoreCloseError(resp.Body.Close()) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("email send failed with status %d", resp.StatusCode)
	}
	return nil
}
