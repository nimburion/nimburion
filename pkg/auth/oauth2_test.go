package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestValidateOAuth2Config_Success(t *testing.T) {
	cfg := OAuth2Config{
		AuthorizeURL: "https://issuer.example.com/authorize",
		TokenURL:     "https://issuer.example.com/oauth/token",
		ClientID:     "client-id",
		RedirectURL:  "https://api.example.com/auth/callback",
		Scopes:       []string{"openid"},
	}

	if err := ValidateOAuth2Config(cfg); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestValidateOAuth2Config_Invalid(t *testing.T) {
	cfg := OAuth2Config{
		AuthorizeURL: "not-a-url",
		TokenURL:     "https://issuer.example.com/oauth/token",
		ClientID:     "",
		RedirectURL:  "https://api.example.com/auth/callback",
		Scopes:       nil,
	}

	if err := ValidateOAuth2Config(cfg); err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestBuildAuthorizeURL_IncludesAudienceWhenSet(t *testing.T) {
	cfg := OAuth2Config{
		AuthorizeURL: "https://issuer.example.com/authorize",
		ClientID:     "client-id",
		RedirectURL:  "https://api.example.com/auth/callback",
		Audience:     "api-audience",
		Scopes:       []string{"openid", "profile", "email"},
	}

	authorizeURL, err := BuildAuthorizeURL(cfg, "state-123")
	if err != nil {
		t.Fatalf("BuildAuthorizeURL returned error: %v", err)
	}

	parsed, err := url.Parse(authorizeURL)
	if err != nil {
		t.Fatalf("failed to parse authorize URL: %v", err)
	}
	query := parsed.Query()

	if got := query.Get("audience"); got != "api-audience" {
		t.Fatalf("expected audience query param, got %q", got)
	}
	if got := query.Get("scope"); got != "openid profile email" {
		t.Fatalf("expected scope query param, got %q", got)
	}
}

func TestExchangeAuthorizationCode_SendsAudienceAndSecret(t *testing.T) {
	var captured url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		captured = r.Form
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"abc","refresh_token":"ref","expires_in":300,"token_type":"Bearer"}`))
	}))
	defer server.Close()

	cfg := OAuth2Config{
		TokenURL:     server.URL,
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://api.example.com/auth/callback",
		Audience:     "api-audience",
	}

	token, err := ExchangeAuthorizationCode(context.Background(), server.Client(), cfg, "code-123")
	if err != nil {
		t.Fatalf("ExchangeAuthorizationCode returned error: %v", err)
	}
	if token.AccessToken != "abc" {
		t.Fatalf("unexpected access token: %s", token.AccessToken)
	}
	if captured.Get("grant_type") != "authorization_code" {
		t.Fatalf("missing or invalid grant_type: %q", captured.Get("grant_type"))
	}
	if captured.Get("audience") != "api-audience" {
		t.Fatalf("missing audience in token request: %q", captured.Get("audience"))
	}
	if captured.Get("client_secret") != "client-secret" {
		t.Fatalf("missing client_secret in token request: %q", captured.Get("client_secret"))
	}
}

func TestExchangeRefreshToken_OmitsOptionalFieldsWhenEmpty(t *testing.T) {
	var captured url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		captured = r.Form
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"abc","expires_in":300,"token_type":"Bearer"}`))
	}))
	defer server.Close()

	cfg := OAuth2Config{
		TokenURL: server.URL,
		ClientID: "client-id",
	}

	if _, err := ExchangeRefreshToken(context.Background(), server.Client(), cfg, "refresh-123"); err != nil {
		t.Fatalf("ExchangeRefreshToken returned error: %v", err)
	}
	if _, ok := captured["audience"]; ok {
		t.Fatal("audience should not be sent when empty")
	}
	if _, ok := captured["client_secret"]; ok {
		t.Fatal("client_secret should not be sent when empty")
	}
}

func TestExchangeAuthorizationCode_ReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"access_denied"}`))
	}))
	defer server.Close()

	cfg := OAuth2Config{
		TokenURL:    server.URL,
		ClientID:    "client-id",
		RedirectURL: "https://api.example.com/auth/callback",
	}

	_, err := ExchangeAuthorizationCode(context.Background(), server.Client(), cfg, "code-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "token endpoint returned 401") {
		t.Fatalf("unexpected error: %v", err)
	}
}

