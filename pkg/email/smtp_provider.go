package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type smtpSendMailFunc func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error

// SMTPConfig configures the SMTP provider.
type SMTPConfig struct {
	Host               string
	Port               int
	Username           string
	Password           string
	From               string
	EnableTLS          bool
	InsecureSkipVerify bool
	OperationTimeout   time.Duration
}

// SMTPProvider sends emails via standard SMTP server.
type SMTPProvider struct {
	cfg      SMTPConfig
	log      logger.Logger
	sendMail smtpSendMailFunc
}

// NewSMTPProvider creates a standard SMTP adapter.
func NewSMTPProvider(cfg SMTPConfig, log logger.Logger) (*SMTPProvider, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, fmt.Errorf("smtp host is required")
	}
	if cfg.Port <= 0 {
		cfg.Port = 587
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &SMTPProvider{
		cfg:      cfg,
		log:      log,
		sendMail: smtp.SendMail,
	}, nil
}

// Send sends email via SMTP.
func (p *SMTPProvider) Send(ctx context.Context, message Message) error {
	msg := message.normalized()
	msg, err := applyDefaultSender(msg, p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.validate(); err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", p.cfg.Host, p.cfg.Port)
	recipients := append(append([]string{}, msg.To...), msg.Cc...)
	recipients = append(recipients, msg.Bcc...)
	raw := buildMIMEMessage(msg)

	var auth smtp.Auth
	if strings.TrimSpace(p.cfg.Username) != "" {
		auth = smtp.PlainAuth("", p.cfg.Username, p.cfg.Password, p.cfg.Host)
	}

	cctx, cancel := withTimeout(ctx, p.cfg.OperationTimeout)
	defer cancel()

	_ = cctx // smtp.SendMail has no context support.
	if p.cfg.EnableTLS && p.cfg.Port == 465 {
		return p.sendMailWithTLS(addr, auth, msg.From, recipients, raw)
	}
	if err := p.sendMail(addr, auth, msg.From, recipients, raw); err != nil {
		return err
	}
	return nil
}

func (p *SMTPProvider) sendMailWithTLS(addr string, auth smtp.Auth, from string, to []string, raw []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		ServerName:         p.cfg.Host,
		InsecureSkipVerify: p.cfg.InsecureSkipVerify,
	})
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, p.cfg.Host)
	if err != nil {
		return err
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// Close releases provider resources.
func (p *SMTPProvider) Close() error {
	return nil
}

func buildMIMEMessage(msg Message) []byte {
	var b strings.Builder
	b.WriteString("From: " + msg.From + "\r\n")
	if len(msg.To) > 0 {
		b.WriteString("To: " + strings.Join(msg.To, ", ") + "\r\n")
	}
	if len(msg.Cc) > 0 {
		b.WriteString("Cc: " + strings.Join(msg.Cc, ", ") + "\r\n")
	}
	if msg.ReplyTo != "" {
		b.WriteString("Reply-To: " + msg.ReplyTo + "\r\n")
	}
	b.WriteString("Subject: " + msg.Subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	for k, v := range msg.Headers {
		key := strings.TrimSpace(k)
		value := strings.TrimSpace(v)
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
