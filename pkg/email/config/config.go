package config

import (
	"fmt"
	"strings"
	"time"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/spf13/viper"
)

const (
	providerSMTP       = "smtp"
	providerSES        = "ses"
	providerSendGrid   = "sendgrid"
	providerMailgun    = "mailgun"
	providerMailchimp  = "mailchimp"
	providerMailerSend = "mailersend"
	providerPostmark   = "postmark"
	providerMailtrap   = "mailtrap"
	providerSMTP2GO    = "smtp2go"
	providerSendPulse  = "sendpulse"
	providerBrevo      = "brevo"
	providerMailjet    = "mailjet"
)

// Config configures pluggable outbound email providers.
type Config struct {
	Enabled  bool   `mapstructure:"enabled"`
	Provider string `mapstructure:"provider"`

	SMTP       SMTPConfig     `mapstructure:"smtp"`
	SES        SESConfig      `mapstructure:"ses"`
	SendGrid   TokenConfig    `mapstructure:"sendgrid"`
	Mailgun    MailgunConfig  `mapstructure:"mailgun"`
	Mailchimp  TokenConfig    `mapstructure:"mailchimp"`
	MailerSend TokenConfig    `mapstructure:"mailersend"`
	Postmark   PostmarkConfig `mapstructure:"postmark"`
	Mailtrap   TokenConfig    `mapstructure:"mailtrap"`
	SMTP2GO    TokenConfig    `mapstructure:"smtp2go"`
	SendPulse  TokenConfig    `mapstructure:"sendpulse"`
	Brevo      TokenConfig    `mapstructure:"brevo"`
	Mailjet    MailjetConfig  `mapstructure:"mailjet"`
}

type TokenConfig struct {
	Token            string        `mapstructure:"token"`
	From             string        `mapstructure:"from"`
	BaseURL          string        `mapstructure:"base_url"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

type SMTPConfig struct {
	Host               string        `mapstructure:"host"`
	Port               int           `mapstructure:"port"`
	Username           string        `mapstructure:"username"`
	Password           string        `mapstructure:"password"`
	From               string        `mapstructure:"from"`
	EnableTLS          bool          `mapstructure:"enable_tls"`
	InsecureSkipVerify bool          `mapstructure:"insecure_skip_verify"`
	OperationTimeout   time.Duration `mapstructure:"operation_timeout"`
}

type SESConfig struct {
	Region           string        `mapstructure:"region"`
	Endpoint         string        `mapstructure:"endpoint"`
	AccessKeyID      string        `mapstructure:"access_key_id"`
	SecretAccessKey  string        `mapstructure:"secret_access_key"`
	SessionToken     string        `mapstructure:"session_token"`
	From             string        `mapstructure:"from"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

type MailgunConfig struct {
	Token            string        `mapstructure:"token"`
	Domain           string        `mapstructure:"domain"`
	From             string        `mapstructure:"from"`
	BaseURL          string        `mapstructure:"base_url"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

type PostmarkConfig struct {
	ServerToken      string        `mapstructure:"server_token"`
	From             string        `mapstructure:"from"`
	BaseURL          string        `mapstructure:"base_url"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

type MailjetConfig struct {
	APIKey           string        `mapstructure:"api_key"`
	APISecret        string        `mapstructure:"api_secret"`
	From             string        `mapstructure:"from"`
	BaseURL          string        `mapstructure:"base_url"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// Extension contributes the email config section as family-owned config surface.
type Extension struct {
	Email Config `mapstructure:"email"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"email"} }

// ApplyDefaults applies family-owned defaults for the email config surface.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("email.enabled", false)
	v.SetDefault("email.provider", providerSMTP)
	v.SetDefault("email.smtp.port", 587)
	v.SetDefault("email.smtp.enable_tls", false)
	v.SetDefault("email.smtp.insecure_skip_verify", false)
	v.SetDefault("email.smtp.operation_timeout", 10*time.Second)
	v.SetDefault("email.ses.operation_timeout", 10*time.Second)
	v.SetDefault("email.sendgrid.base_url", "https://api.sendgrid.com")
	v.SetDefault("email.sendgrid.operation_timeout", 10*time.Second)
	v.SetDefault("email.mailgun.base_url", "https://api.mailgun.net")
	v.SetDefault("email.mailgun.operation_timeout", 10*time.Second)
	v.SetDefault("email.mailchimp.base_url", "https://mandrillapp.com/api/1.0")
	v.SetDefault("email.mailchimp.operation_timeout", 10*time.Second)
	v.SetDefault("email.mailersend.base_url", "https://api.mailersend.com")
	v.SetDefault("email.mailersend.operation_timeout", 10*time.Second)
	v.SetDefault("email.postmark.base_url", "https://api.postmarkapp.com")
	v.SetDefault("email.postmark.operation_timeout", 10*time.Second)
	v.SetDefault("email.mailtrap.base_url", "https://send.api.mailtrap.io")
	v.SetDefault("email.mailtrap.operation_timeout", 10*time.Second)
	v.SetDefault("email.smtp2go.base_url", "https://api.smtp2go.com")
	v.SetDefault("email.smtp2go.operation_timeout", 10*time.Second)
	v.SetDefault("email.sendpulse.base_url", "https://api.sendpulse.com")
	v.SetDefault("email.sendpulse.operation_timeout", 10*time.Second)
	v.SetDefault("email.brevo.base_url", "https://api.brevo.com")
	v.SetDefault("email.brevo.operation_timeout", 10*time.Second)
	v.SetDefault("email.mailjet.base_url", "https://api.mailjet.com")
	v.SetDefault("email.mailjet.operation_timeout", 10*time.Second)
}

// BindEnv binds family-owned environment variables for the email config surface.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	bind := func(key string, suffixes ...string) error {
		envs := make([]string, 0, len(suffixes))
		for _, suffix := range suffixes {
			envs = append(envs, prefixedEnv(prefix, suffix))
		}
		return v.BindEnv(append([]string{key}, envs...)...)
	}

	pairs := []struct {
		key   string
		suffs []string
	}{
		{"email.enabled", []string{"EMAIL_ENABLED"}},
		{"email.provider", []string{"EMAIL_PROVIDER"}},
		{"email.smtp.host", []string{"EMAIL_SMTP_HOST"}},
		{"email.smtp.port", []string{"EMAIL_SMTP_PORT"}},
		{"email.smtp.username", []string{"EMAIL_SMTP_USERNAME"}},
		{"email.smtp.password", []string{"EMAIL_SMTP_PASSWORD"}},
		{"email.smtp.from", []string{"EMAIL_SMTP_FROM"}},
		{"email.smtp.enable_tls", []string{"EMAIL_SMTP_ENABLE_TLS"}},
		{"email.smtp.insecure_skip_verify", []string{"EMAIL_SMTP_INSECURE_SKIP_VERIFY"}},
		{"email.smtp.operation_timeout", []string{"EMAIL_SMTP_OPERATION_TIMEOUT"}},
		{"email.ses.region", []string{"EMAIL_SES_REGION"}},
		{"email.ses.endpoint", []string{"EMAIL_SES_ENDPOINT"}},
		{"email.ses.access_key_id", []string{"EMAIL_SES_ACCESS_KEY_ID"}},
		{"email.ses.secret_access_key", []string{"EMAIL_SES_SECRET_ACCESS_KEY"}},
		{"email.ses.session_token", []string{"EMAIL_SES_SESSION_TOKEN"}},
		{"email.ses.from", []string{"EMAIL_SES_FROM"}},
		{"email.ses.operation_timeout", []string{"EMAIL_SES_OPERATION_TIMEOUT"}},
		{"email.sendgrid.token", []string{"EMAIL_SENDGRID_TOKEN"}},
		{"email.sendgrid.from", []string{"EMAIL_SENDGRID_FROM"}},
		{"email.sendgrid.base_url", []string{"EMAIL_SENDGRID_BASE_URL"}},
		{"email.sendgrid.operation_timeout", []string{"EMAIL_SENDGRID_OPERATION_TIMEOUT"}},
		{"email.mailgun.token", []string{"EMAIL_MAILGUN_TOKEN"}},
		{"email.mailgun.domain", []string{"EMAIL_MAILGUN_DOMAIN"}},
		{"email.mailgun.from", []string{"EMAIL_MAILGUN_FROM"}},
		{"email.mailgun.base_url", []string{"EMAIL_MAILGUN_BASE_URL"}},
		{"email.mailgun.operation_timeout", []string{"EMAIL_MAILGUN_OPERATION_TIMEOUT"}},
		{"email.mailchimp.token", []string{"EMAIL_MAILCHIMP_TOKEN"}},
		{"email.mailchimp.from", []string{"EMAIL_MAILCHIMP_FROM"}},
		{"email.mailchimp.base_url", []string{"EMAIL_MAILCHIMP_BASE_URL"}},
		{"email.mailchimp.operation_timeout", []string{"EMAIL_MAILCHIMP_OPERATION_TIMEOUT"}},
		{"email.mailersend.token", []string{"EMAIL_MAILERSEND_TOKEN"}},
		{"email.mailersend.from", []string{"EMAIL_MAILERSEND_FROM"}},
		{"email.mailersend.base_url", []string{"EMAIL_MAILERSEND_BASE_URL"}},
		{"email.mailersend.operation_timeout", []string{"EMAIL_MAILERSEND_OPERATION_TIMEOUT"}},
		{"email.postmark.server_token", []string{"EMAIL_POSTMARK_SERVER_TOKEN"}},
		{"email.postmark.from", []string{"EMAIL_POSTMARK_FROM"}},
		{"email.postmark.base_url", []string{"EMAIL_POSTMARK_BASE_URL"}},
		{"email.postmark.operation_timeout", []string{"EMAIL_POSTMARK_OPERATION_TIMEOUT"}},
		{"email.mailtrap.token", []string{"EMAIL_MAILTRAP_TOKEN"}},
		{"email.mailtrap.from", []string{"EMAIL_MAILTRAP_FROM"}},
		{"email.mailtrap.base_url", []string{"EMAIL_MAILTRAP_BASE_URL"}},
		{"email.mailtrap.operation_timeout", []string{"EMAIL_MAILTRAP_OPERATION_TIMEOUT"}},
		{"email.smtp2go.token", []string{"EMAIL_SMTP2GO_TOKEN"}},
		{"email.smtp2go.from", []string{"EMAIL_SMTP2GO_FROM"}},
		{"email.smtp2go.base_url", []string{"EMAIL_SMTP2GO_BASE_URL"}},
		{"email.smtp2go.operation_timeout", []string{"EMAIL_SMTP2GO_OPERATION_TIMEOUT"}},
		{"email.sendpulse.token", []string{"EMAIL_SENDPULSE_TOKEN"}},
		{"email.sendpulse.from", []string{"EMAIL_SENDPULSE_FROM"}},
		{"email.sendpulse.base_url", []string{"EMAIL_SENDPULSE_BASE_URL"}},
		{"email.sendpulse.operation_timeout", []string{"EMAIL_SENDPULSE_OPERATION_TIMEOUT"}},
		{"email.brevo.token", []string{"EMAIL_BREVO_TOKEN"}},
		{"email.brevo.from", []string{"EMAIL_BREVO_FROM"}},
		{"email.brevo.base_url", []string{"EMAIL_BREVO_BASE_URL"}},
		{"email.brevo.operation_timeout", []string{"EMAIL_BREVO_OPERATION_TIMEOUT"}},
		{"email.mailjet.api_key", []string{"EMAIL_MAILJET_API_KEY"}},
		{"email.mailjet.api_secret", []string{"EMAIL_MAILJET_API_SECRET"}},
		{"email.mailjet.from", []string{"EMAIL_MAILJET_FROM"}},
		{"email.mailjet.base_url", []string{"EMAIL_MAILJET_BASE_URL"}},
		{"email.mailjet.operation_timeout", []string{"EMAIL_MAILJET_OPERATION_TIMEOUT"}},
	}
	for _, pair := range pairs {
		if err := bind(pair.key, pair.suffs...); err != nil {
			return err
		}
	}
	return nil
}

// Validate validates family-owned email configuration.
func (e Extension) Validate() error {
	if !e.Email.Enabled {
		return nil
	}

	provider := strings.ToLower(strings.TrimSpace(e.Email.Provider))
	validProviders := []string{
		providerSMTP,
		providerSES,
		providerSendGrid,
		providerMailgun,
		providerMailchimp,
		providerMailerSend,
		providerPostmark,
		providerMailtrap,
		providerSMTP2GO,
		providerSendPulse,
		providerBrevo,
		providerMailjet,
	}
	if !contains(validProviders, provider) {
		return validationErrorf("validation.email.provider.invalid", "invalid email.provider: %s (must be one of: %v)", e.Email.Provider, validProviders)
	}

	switch provider {
	case providerSMTP:
		if strings.TrimSpace(e.Email.SMTP.Host) == "" {
			return validationError("validation.email.smtp.host.required", "email.smtp.host is required when email.provider=smtp")
		}
	case providerSES:
		if strings.TrimSpace(e.Email.SES.Region) == "" {
			return validationError("validation.email.ses.region.required", "email.ses.region is required when email.provider=ses")
		}
	case providerSendGrid:
		if strings.TrimSpace(e.Email.SendGrid.Token) == "" {
			return validationError("validation.email.sendgrid.token.required", "email.sendgrid.token is required when email.provider=sendgrid")
		}
	case providerMailgun:
		if strings.TrimSpace(e.Email.Mailgun.Token) == "" {
			return validationError("validation.email.mailgun.token.required", "email.mailgun.token is required when email.provider=mailgun")
		}
		if strings.TrimSpace(e.Email.Mailgun.Domain) == "" {
			return validationError("validation.email.mailgun.domain.required", "email.mailgun.domain is required when email.provider=mailgun")
		}
	case providerMailchimp:
		if strings.TrimSpace(e.Email.Mailchimp.Token) == "" {
			return validationError("validation.email.mailchimp.token.required", "email.mailchimp.token is required when email.provider=mailchimp")
		}
	case providerMailerSend:
		if strings.TrimSpace(e.Email.MailerSend.Token) == "" {
			return validationError("validation.email.mailersend.token.required", "email.mailersend.token is required when email.provider=mailersend")
		}
	case providerPostmark:
		if strings.TrimSpace(e.Email.Postmark.ServerToken) == "" {
			return validationError("validation.email.postmark.server_token.required", "email.postmark.server_token is required when email.provider=postmark")
		}
	case providerMailtrap:
		if strings.TrimSpace(e.Email.Mailtrap.Token) == "" {
			return validationError("validation.email.mailtrap.token.required", "email.mailtrap.token is required when email.provider=mailtrap")
		}
	case providerSMTP2GO:
		if strings.TrimSpace(e.Email.SMTP2GO.Token) == "" {
			return validationError("validation.email.smtp2go.token.required", "email.smtp2go.token is required when email.provider=smtp2go")
		}
	case providerSendPulse:
		if strings.TrimSpace(e.Email.SendPulse.Token) == "" {
			return validationError("validation.email.sendpulse.token.required", "email.sendpulse.token is required when email.provider=sendpulse")
		}
	case providerBrevo:
		if strings.TrimSpace(e.Email.Brevo.Token) == "" {
			return validationError("validation.email.brevo.token.required", "email.brevo.token is required when email.provider=brevo")
		}
	case providerMailjet:
		if strings.TrimSpace(e.Email.Mailjet.APIKey) == "" {
			return validationError("validation.email.mailjet.api_key.required", "email.mailjet.api_key is required when email.provider=mailjet")
		}
		if strings.TrimSpace(e.Email.Mailjet.APISecret) == "" {
			return validationError("validation.email.mailjet.api_secret.required", "email.mailjet.api_secret is required when email.provider=mailjet")
		}
	}

	return nil
}

func validationError(code, message string) error {
	return coreerrors.NewValidationWithCode(code, message, nil, nil)
}

func validationErrorf(code, format string, args ...any) error {
	return validationError(code, fmt.Sprintf(format, args...))
}

func prefixedEnv(prefix, suffix string) string {
	trimmedPrefix := strings.TrimSpace(prefix)
	trimmedSuffix := strings.TrimSpace(suffix)
	if trimmedPrefix == "" {
		return trimmedSuffix
	}
	return trimmedPrefix + "_" + trimmedSuffix
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
