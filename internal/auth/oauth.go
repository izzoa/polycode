package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceFlowConfig holds the parameters for an OAuth 2.0 device authorization flow.
type DeviceFlowConfig struct {
	ClientID      string
	TokenURL      string
	DeviceAuthURL string
	Scopes        []string
}

// deviceAuthResponse is the response from the device authorization endpoint.
type deviceAuthResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// tokenResponse is the response from the token endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// RunDeviceFlow executes the OAuth 2.0 device authorization grant flow.
// It prints a verification URL and user code, then polls until the user
// authorizes the application (or the flow times out). On success the access
// token is persisted via the provided Store and returned.
func RunDeviceFlow(cfg DeviceFlowConfig, store Store) (string, error) {
	// Step 1: Request device and user codes.
	authResp, err := requestDeviceCode(cfg)
	if err != nil {
		return "", fmt.Errorf("device auth request: %w", err)
	}

	// Step 2: Prompt the user.
	fmt.Println()
	fmt.Printf("  Open this URL in your browser:  %s\n", authResp.VerificationURI)
	fmt.Printf("  Enter the code:                 %s\n", authResp.UserCode)
	fmt.Println()

	// Step 3: Poll for the token.
	interval := time.Duration(authResp.Interval) * time.Second
	if interval == 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second)

	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("device flow timed out after %d seconds", authResp.ExpiresIn)
		}

		time.Sleep(interval)

		token, err := pollToken(cfg, authResp.DeviceCode)
		if err != nil {
			return "", err
		}

		if token.Error == "authorization_pending" {
			continue
		}
		if token.Error == "slow_down" {
			interval += 5 * time.Second
			continue
		}
		if token.Error != "" {
			return "", fmt.Errorf("token error: %s — %s", token.Error, token.ErrorDesc)
		}

		// Success — store the token.
		if err := store.Set(cfg.ClientID, token.AccessToken); err != nil {
			return "", fmt.Errorf("storing access token: %w", err)
		}
		if token.RefreshToken != "" {
			if err := store.Set(cfg.ClientID+":refresh", token.RefreshToken); err != nil {
				return "", fmt.Errorf("storing refresh token: %w", err)
			}
		}

		return token.AccessToken, nil
	}
}

// RefreshToken exchanges a refresh token for a new access token and persists
// the result in the store.
func RefreshToken(cfg DeviceFlowConfig, store Store, refreshToken string) (string, error) {
	data := url.Values{
		"client_id":     {cfg.ClientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	resp, err := http.PostForm(cfg.TokenURL, data)
	if err != nil {
		return "", fmt.Errorf("refresh token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading refresh response: %w", err)
	}

	var token tokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return "", fmt.Errorf("parsing refresh response: %w", err)
	}

	if token.Error != "" {
		return "", fmt.Errorf("refresh error: %s — %s", token.Error, token.ErrorDesc)
	}

	if err := store.Set(cfg.ClientID, token.AccessToken); err != nil {
		return "", fmt.Errorf("storing refreshed access token: %w", err)
	}
	if token.RefreshToken != "" {
		if err := store.Set(cfg.ClientID+":refresh", token.RefreshToken); err != nil {
			return "", fmt.Errorf("storing rotated refresh token: %w", err)
		}
	}

	return token.AccessToken, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func requestDeviceCode(cfg DeviceFlowConfig) (*deviceAuthResponse, error) {
	data := url.Values{
		"client_id": {cfg.ClientID},
	}
	if len(cfg.Scopes) > 0 {
		data.Set("scope", strings.Join(cfg.Scopes, " "))
	}

	resp, err := http.PostForm(cfg.DeviceAuthURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading device auth response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device auth endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var authResp deviceAuthResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return nil, fmt.Errorf("parsing device auth response: %w", err)
	}

	if authResp.DeviceCode == "" || authResp.UserCode == "" {
		return nil, fmt.Errorf("device auth response missing required fields: %s", string(body))
	}

	return &authResp, nil
}

func pollToken(cfg DeviceFlowConfig, deviceCode string) (*tokenResponse, error) {
	data := url.Values{
		"client_id":   {cfg.ClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	resp, err := http.PostForm(cfg.TokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token poll request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}

	var token tokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}

	return &token, nil
}
