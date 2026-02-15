package email

import (
	"fmt"
	"strings"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

const (
	ProviderSMTP       = "smtp"
	ProviderSES        = "ses"
	ProviderSendGrid   = "sendgrid"
	ProviderMailgun    = "mailgun"
	ProviderMailchimp  = "mailchimp"
	ProviderMailerSend = "mailersend"
	ProviderPostmark   = "postmark"
	ProviderMailtrap   = "mailtrap"
	ProviderSMTP2GO    = "smtp2go"
	ProviderSendPulse  = "sendpulse"
	ProviderBrevo      = "brevo"
	ProviderMailjet    = "mailjet"
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
