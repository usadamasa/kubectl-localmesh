package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// TokenSource はADC (Application Default Credentials) からOAuth2トークンソースを取得します。
// scope: https://www.googleapis.com/auth/cloud-platform
func TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	ts, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("failed to get default token source: %w", err)
	}
	return ts, nil
}

// ResolveOSLoginUser はGCP OS Login APIを使ってPOSIXユーザー名を解決します。
// OS Loginが有効な環境では、SSHユーザー名はGoogleアカウントのメールアドレスから
// 自動生成されます（例: masaru.uchida@example.com → masaru_uchida_example_com）。
func ResolveOSLoginUser(ctx context.Context, ts oauth2.TokenSource) (string, error) {
	client := oauth2.NewClient(ctx, ts)

	// 1. ADCのメールアドレスを取得
	email, err := getAuthenticatedEmail(ctx, client)
	if err != nil {
		return "", fmt.Errorf("failed to get authenticated email: %w", err)
	}

	// 2. OS Login APIでPOSIXユーザー名を取得
	username, err := getOSLoginUsername(ctx, client, email)
	if err != nil {
		return "", fmt.Errorf("failed to get OS Login username for %s: %w", email, err)
	}

	return username, nil
}

// getAuthenticatedEmail はOAuth2 userinfoエンドポイントから認証済みユーザーのメールを取得します。
func getAuthenticatedEmail(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo request failed with status %d", resp.StatusCode)
	}

	var info struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	if info.Email == "" {
		return "", fmt.Errorf("email not found in userinfo response")
	}

	return info.Email, nil
}

// getOSLoginUsername はOS Login APIからPOSIXユーザー名を取得します。
func getOSLoginUsername(ctx context.Context, client *http.Client, email string) (string, error) {
	url := fmt.Sprintf("https://oslogin.googleapis.com/v1/users/%s/loginProfile", email)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OS Login API returned status %d", resp.StatusCode)
	}

	var profile struct {
		PosixAccounts []struct {
			Username string `json:"username"`
		} `json:"posixAccounts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return "", err
	}

	if len(profile.PosixAccounts) == 0 || profile.PosixAccounts[0].Username == "" {
		return "", fmt.Errorf("no POSIX account found")
	}

	return profile.PosixAccounts[0].Username, nil
}
