package smtp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	gosmtp "net/smtp"
	"strings"
	"time"

	"github.com/nimburion/nimburion/internal/emailkit"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/email"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type sendMailFunc func(addr string, auth gosmtp.Auth, from string, to []string, msg []byte) error

type Config = emailconfig.SMTPConfig

type Provider struct {
	cfg      Config
	log      logger.Logger
	sendMail sendMailFunc
}

func New(cfg Config, log logger.Logger) (*Provider, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, coreerrors.NewValidationWithCode("validation.email.smtp.host.required", "smtp host is required", nil, nil)
	}
	if cfg.Port <= 0 {
		cfg.Port = 587
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &Provider{cfg: cfg, log: log, sendMail: gosmtp.SendMail}, nil
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
	addr := fmt.Sprintf("%s:%d", p.cfg.Host, p.cfg.Port)
	recipients := append(append([]string{}, msg.To...), msg.Cc...)
	recipients = append(recipients, msg.Bcc...)
	raw := emailkit.BuildMIMEMessage(msg)
	var auth gosmtp.Auth
	if strings.TrimSpace(p.cfg.Username) != "" {
		auth = gosmtp.PlainAuth("", p.cfg.Username, p.cfg.Password, p.cfg.Host)
	}
	cctx, cancel := emailkit.WithTimeout(ctx, p.cfg.OperationTimeout)
	defer cancel()
	_ = cctx
	if p.cfg.EnableTLS && p.cfg.Port == 465 {
		return p.sendMailWithTLS(addr, auth, msg.From, recipients, raw)
	}
	return p.sendMail(addr, auth, msg.From, recipients, raw)
}

func (p *Provider) sendMailWithTLS(addr string, auth gosmtp.Auth, from string, to []string, raw []byte) error {
	tlsConfig := &tls.Config{ServerName: p.cfg.Host, MinVersion: tls.VersionTLS12}
	if p.cfg.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	var sendErr error
	defer func() {
		if closeErr := conn.Close(); sendErr == nil && closeErr != nil {
			sendErr = closeErr
		}
	}()
	client, err := gosmtp.NewClient(conn, p.cfg.Host)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			sendErr = errors.Join(sendErr, closeErr)
		}
	}()
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
	if err := client.Quit(); err != nil {
		return err
	}
	return sendErr
}

func (p *Provider) Close() error { return nil }
