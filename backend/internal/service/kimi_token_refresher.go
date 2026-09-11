package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

const kimiTokenRefreshSkew = 5 * time.Minute

type KimiTokenRefresher struct {
	kimiOAuthService KimiOAuthTokenService
}

func NewKimiTokenRefresher(kimiOAuthService KimiOAuthTokenService) *KimiTokenRefresher {
	return &KimiTokenRefresher{kimiOAuthService: kimiOAuthService}
}

func (r *KimiTokenRefresher) CacheKey(account *Account) string {
	return KimiTokenCacheKey(account)
}

func (r *KimiTokenRefresher) CanRefresh(account *Account) bool {
	return account != nil && account.IsKimiOAuth() && strings.TrimSpace(account.GetKimiRefreshToken()) != ""
}

func (r *KimiTokenRefresher) NeedsRefresh(account *Account, refreshWindow time.Duration) bool {
	if !r.CanRefresh(account) {
		return false
	}
	if strings.TrimSpace(account.GetKimiAccessToken()) == "" {
		return true
	}
	expiresAt := account.GetCredentialAsTime("expires_at")
	if expiresAt == nil {
		return true
	}
	if refreshWindow < kimiTokenRefreshSkew {
		refreshWindow = kimiTokenRefreshSkew
	}
	return time.Until(*expiresAt) < refreshWindow
}

func (r *KimiTokenRefresher) Refresh(ctx context.Context, account *Account) (map[string]any, error) {
	if r == nil || r.kimiOAuthService == nil {
		return nil, errors.New("kimi oauth service is not configured")
	}
	tokenInfo, err := r.kimiOAuthService.RefreshAccountToken(ctx, account)
	if err != nil {
		return nil, err
	}
	newCredentials := r.kimiOAuthService.BuildAccountCredentials(tokenInfo)
	newCredentials = MergeCredentials(account.Credentials, newCredentials)
	if baseURL := strings.TrimSpace(account.GetCredential("base_url")); baseURL != "" {
		newCredentials["base_url"] = baseURL
	}
	if protocol := strings.TrimSpace(account.GetCredential("api_protocol")); protocol != "" {
		newCredentials["api_protocol"] = protocol
	}
	if raw, ok := account.Credentials["api_base_urls"]; ok {
		newCredentials["api_base_urls"] = raw
	}
	return newCredentials, nil
}
