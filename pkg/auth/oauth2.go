package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// OAuth2Config is a provider-agnostic OAuth2 configuration used for
// authorization-code and refresh-token flows.
type OAuth2Config struct {
	AuthorizeURL string
	TokenURL     string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Audience     string
	Scopes       []string
}

// OAuth2TokenResponse models a standard token endpoint response.
type OAuth2TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
}

// ValidateOAuth2Config validates the minimum required OAuth2 configuration.
func ValidateOAuth2Config(cfg OAuth2Config) error {
	required := map[string]string{
		"authorize_url": cfg.AuthorizeURL,
		"token_url":     cfg.TokenURL,
		"client_id":     cfg.ClientID,
		"redirect_url":  cfg.RedirectURL,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("oauth2.%s is required", key)
		}
	}
	if len(cfg.Scopes) == 0 {
		return errors.New("oauth2.scopes must contain at least one scope")
	}
	if !isValidAbsoluteURL(cfg.AuthorizeURL) {
		return errors.New("oauth2.authorize_url must be a valid absolute URL")
	}
	if !isValidAbsoluteURL(cfg.TokenURL) {
		return errors.New("oauth2.token_url must be a valid absolute URL")
	}
	if !isValidAbsoluteURL(cfg.RedirectURL) {
		return errors.New("oauth2.redirect_url must be a valid absolute URL")
	}
	return nil
}

// BuildAuthorizeURL builds an OAuth2 authorization URL with common query
// parameters. Audience is included only when configured.
func BuildAuthorizeURL(cfg OAuth2Config, state string) (string, error) {
	parsed, err := url.Parse(cfg.AuthorizeURL)
	if err != nil {
		return "", err
	}

	values := parsed.Query()
	values.Set("response_type", "code")
	values.Set("client_id", cfg.ClientID)
	values.Set("redirect_uri", cfg.RedirectURL)
	values.Set("scope", strings.Join(cfg.Scopes, " "))
	values.Set("state", state)
	if strings.TrimSpace(cfg.Audience) != "" {
		values.Set("audience", cfg.Audience)
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

// ExchangeAuthorizationCode exchanges an authorization code for tokens.
func ExchangeAuthorizationCode(ctx context.Context, httpClient *http.Client, cfg OAuth2Config, code string) (*OAuth2TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", cfg.RedirectURL)
	form.Set("client_id", cfg.ClientID)
	if strings.TrimSpace(cfg.Audience) != "" {
		form.Set("audience", cfg.Audience)
	}
	if strings.TrimSpace(cfg.ClientSecret) != "" {
		form.Set("client_secret", cfg.ClientSecret)
	}
	return exchangeToken(ctx, httpClient, cfg.TokenURL, form)
}

// ExchangeRefreshToken exchanges a refresh token for a new access token.
func ExchangeRefreshToken(ctx context.Context, httpClient *http.Client, cfg OAuth2Config, refreshToken string) (*OAuth2TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", cfg.ClientID)
	if strings.TrimSpace(cfg.Audience) != "" {
		form.Set("audience", cfg.Audience)
	}
	if strings.TrimSpace(cfg.ClientSecret) != "" {
		form.Set("client_secret", cfg.ClientSecret)
	}
	return exchangeToken(ctx, httpClient, cfg.TokenURL, form)
}

func exchangeToken(ctx context.Context, httpClient *http.Client, tokenURL string, form url.Values) (*OAuth2TokenResponse, error) {
	client := httpClient
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var token OAuth2TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return nil, errors.New("token endpoint did not return access_token")
	}
	return &token, nil
}

func isValidAbsoluteURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme != "" && parsed.Host != ""
}
