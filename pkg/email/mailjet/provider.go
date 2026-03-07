package mailjet

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nimburion/nimburion/internal/emailkit"
	"github.com/nimburion/nimburion/pkg/email"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type Config = emailconfig.MailjetConfig
type Provider struct {
	cfg        Config
	httpClient *http.Client
	log        logger.Logger
}

func New(cfg Config, log logger.Logger) (*Provider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("mailjet api key is required")
	}
	if strings.TrimSpace(cfg.APISecret) == "" {
		return nil, fmt.Errorf("mailjet api secret is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.mailjet.com"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	return &Provider{cfg: cfg, httpClient: emailkit.DefaultHTTPClient(nil, cfg.OperationTimeout), log: log}, nil
}
func (p *Provider) Send(ctx context.Context, message email.Message) error {
	msg, err := email.ApplyDefaultSender(message.Normalized(), p.cfg.From)
	if err != nil {
		return err
	}
	if err := msg.Validate(); err != nil {
		return err
	}
	payload := map[string]interface{}{"Messages": []map[string]interface{}{{"From": map[string]string{"Email": msg.From}, "To": emailkit.MapRecipientsWithKey(msg.To, "Email"), "Cc": emailkit.MapRecipientsWithKey(msg.Cc, "Email"), "Bcc": emailkit.MapRecipientsWithKey(msg.Bcc, "Email"), "Subject": msg.Subject, "TextPart": msg.TextBody, "HTMLPart": msg.HTMLBody}}}
	auth := base64.StdEncoding.EncodeToString([]byte(p.cfg.APIKey + ":" + p.cfg.APISecret))
	return emailkit.SendJSON(ctx, p.httpClient, p.cfg.OperationTimeout, strings.TrimRight(p.cfg.BaseURL, "/")+"/v3.1/send", payload, map[string]string{"Authorization": "Basic " + auth})
}
func (p *Provider) Close() error { return nil }
