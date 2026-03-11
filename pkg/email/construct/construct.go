// Package construct builds email providers from framework configuration.
package construct

import (
	"strings"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/email"
	"github.com/nimburion/nimburion/pkg/email/brevo"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
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
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// NewProvider constructs an email provider from framework configuration.
func NewProvider(cfg emailconfig.Config, log logger.Logger) (email.Provider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "smtp":
		return smtp.New(cfg.SMTP, log)
	case "ses":
		return ses.New(cfg.SES, log)
	case "sendgrid":
		return sendgrid.New(cfg.SendGrid, log)
	case "mailgun":
		return mailgun.New(cfg.Mailgun, log)
	case "mailchimp":
		return mailchimp.New(cfg.Mailchimp, log)
	case "mailersend":
		return mailersend.New(cfg.MailerSend, log)
	case "postmark":
		return postmark.New(cfg.Postmark, log)
	case "mailtrap":
		return mailtrap.New(cfg.Mailtrap, log)
	case "smtp2go":
		return smtp2go.New(cfg.SMTP2GO, log)
	case "sendpulse":
		return sendpulse.New(cfg.SendPulse, log)
	case "brevo":
		return brevo.New(cfg.Brevo, log)
	case "mailjet":
		return mailjet.New(cfg.Mailjet, log)
	default:
		return nil, coreerrors.NewValidationWithCode(
			"validation.email.provider.unsupported",
			"unsupported email provider "+`"`+cfg.Provider+`"`,
			nil,
			map[string]interface{}{"provider": cfg.Provider},
		)
	}
}
