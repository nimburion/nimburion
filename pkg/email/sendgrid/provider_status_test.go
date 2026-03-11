package sendgrid

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/email"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestSend_StatusErrorIsTyped(t *testing.T) {
	p := &Provider{
		cfg: emailconfig.TokenConfig{
			Token:            "sg",
			From:             "noreply@example.com",
			BaseURL:          "https://example.com",
			OperationTimeout: time.Second,
		},
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Body:       io.NopCloser(strings.NewReader("fail")),
					Header:     make(http.Header),
					Request:    req,
				}, nil
			}),
		},
	}

	err := p.Send(context.Background(), email.Message{To: []string{"u@example.com"}, Subject: "hello", TextBody: "body"})
	if err == nil {
		t.Fatal("expected send error")
	}
	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != coreerrors.CodeUnavailable {
		t.Fatalf("Code = %q", appErr.Code)
	}
}
