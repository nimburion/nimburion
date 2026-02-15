package email

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Provider is a pluggable email sender implementation.
type Provider interface {
	Send(ctx context.Context, message Message) error
	Close() error
}

// Message is the normalized email payload accepted by all providers.
type Message struct {
	From     string
	To       []string
	Cc       []string
	Bcc      []string
	ReplyTo  string
	Subject  string
	TextBody string
	HTMLBody string
	Headers  map[string]string
}

func (m Message) normalized() Message {
	cp := m
	cp.From = strings.TrimSpace(cp.From)
	cp.ReplyTo = strings.TrimSpace(cp.ReplyTo)
	cp.Subject = strings.TrimSpace(cp.Subject)
	cp.To = normalizeEmailList(cp.To)
	cp.Cc = normalizeEmailList(cp.Cc)
	cp.Bcc = normalizeEmailList(cp.Bcc)
	return cp
}

func (m Message) validate() error {
	totalRecipients := len(m.To) + len(m.Cc) + len(m.Bcc)
	if totalRecipients == 0 {
		return errors.New("at least one recipient is required")
	}
	if strings.TrimSpace(m.Subject) == "" {
		return errors.New("email subject is required")
	}
	if strings.TrimSpace(m.TextBody) == "" && strings.TrimSpace(m.HTMLBody) == "" {
		return errors.New("email body is required (text or html)")
	}
	return nil
}

func normalizeEmailList(list []string) []string {
	if len(list) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(list))
	out := make([]string, 0, len(list))
	for _, value := range list {
		email := strings.TrimSpace(value)
		if email == "" {
			continue
		}
		key := strings.ToLower(email)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, email)
	}
	return out
}

func applyDefaultSender(message Message, defaultFrom string) (Message, error) {
	if strings.TrimSpace(message.From) != "" {
		return message, nil
	}
	defaultFrom = strings.TrimSpace(defaultFrom)
	if defaultFrom == "" {
		return Message{}, fmt.Errorf("message.from is required when provider default sender is empty")
	}
	message.From = defaultFrom
	return message, nil
}
