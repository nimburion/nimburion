package email

import (
	"fmt"
	"strings"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// Email provider type constants
const (
	// ProviderSMTP uses standard SMTP protocol
	ProviderSMTP = "smtp"
	// ProviderSES uses AWS Simple Email Service
	ProviderSES = "ses"
	// ProviderSendGrid uses SendGrid API
	ProviderSendGrid = "sendgrid"
	// ProviderMailgun uses Mailgun API
	ProviderMailgun = "mailgun"
	// ProviderMailchimp uses Mailchimp Transactional API
	ProviderMailchimp = "mailchimp"
	// ProviderMailerSend uses MailerSend API
	ProviderMailerSend = "mailersend"
	// ProviderPostmark uses Postmark API
	ProviderPostmark = "postmark"
	// ProviderMailtrap uses Mailtrap API
	ProviderMailtrap = "mailtrap"
	// ProviderSMTP2GO uses SMTP2GO API
	ProviderSMTP2GO = "smtp2go"
	// ProviderSendPulse uses SendPulse API
	ProviderSendPulse = "sendpulse"
	// ProviderBrevo uses Brevo (formerly Sendinblue) API
	ProviderBrevo = "brevo"
	// ProviderMailjet uses Mailjet API
	ProviderMailjet = "mailjet"
)

// Config is the root provider factory configuration.
type Config struct {
	Provider string

	SMTP       SMTPConfig
	SES        SESConfig
	SendGrid   SendGridConfig
	Mailgun    MailgunConfig
	Mailchimp  MailchimpConfig
	MailerSend MailerSendConfig
	Postmark   PostmarkConfig
	Mailtrap   MailtrapConfig
	SMTP2GO    SMTP2GOConfig
	SendPulse  SendPulseConfig
	Brevo      BrevoConfig
	Mailjet    MailjetConfig
}

// NewProvider creates an email provider adapter from configuration.
func NewProvider(cfg Config, log logger.Logger) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case ProviderSMTP:
		return NewSMTPProvider(cfg.SMTP, log)
	case ProviderSES:
		return NewSESProvider(cfg.SES, log)
	case ProviderSendGrid:
		return NewSendGridProvider(cfg.SendGrid, log)
	case ProviderMailgun:
		return NewMailgunProvider(cfg.Mailgun, log)
	case ProviderMailchimp:
		return NewMailchimpProvider(cfg.Mailchimp, log)
	case ProviderMailerSend:
		return NewMailerSendProvider(cfg.MailerSend, log)
	case ProviderPostmark:
		return NewPostmarkProvider(cfg.Postmark, log)
	case ProviderMailtrap:
		return NewMailtrapProvider(cfg.Mailtrap, log)
	case ProviderSMTP2GO:
		return NewSMTP2GOProvider(cfg.SMTP2GO, log)
	case ProviderSendPulse:
		return NewSendPulseProvider(cfg.SendPulse, log)
	case ProviderBrevo:
		return NewBrevoProvider(cfg.Brevo, log)
	case ProviderMailjet:
		return NewMailjetProvider(cfg.Mailjet, log)
	default:
		return nil, fmt.Errorf(
			"unsupported email provider %q (supported: smtp, ses, sendgrid, mailgun, mailchimp, mailersend, postmark, mailtrap, smtp2go, sendpulse, brevo, mailjet)",
			cfg.Provider,
		)
	}
}
