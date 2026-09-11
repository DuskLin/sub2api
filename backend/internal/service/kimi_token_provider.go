package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const (
	kimiTokenCacheSkew        = 5 * time.Minute
	kimiRequestRefreshTimeout = 8 * time.Second
	kimiLockWaitTime          = 200 * time.Millisecond
)

type KimiTokenCache = GeminiTokenCache

type KimiTokenProvider struct {
	accountRepo   AccountRepository
	tokenCache    KimiTokenCache
	refreshAPI    *OAuthRefreshAPI
	executor      OAuthRefreshExecutor
	refreshPolicy ProviderRefreshPolicy
}

func NewKimiTokenProvider(accountRepo AccountRepository, tokenCache KimiTokenCache) *KimiTokenProvider {
	return &KimiTokenProvider{
		accountRepo:   accountRepo,
		tokenCache:    tokenCache,
		refreshPolicy: ClaudeProviderRefreshPolicy(),
	}
}

func (p *KimiTokenProvider) SetRefreshAPI(api *OAuthRefreshAPI, executor OAuthRefreshExecutor) {
	p.refreshAPI = api
	p.executor = executor
}

func (p *KimiTokenProvider) SetRefreshPolicy(policy ProviderRefreshPolicy) {
	p.refreshPolicy = policy
}

func (p *KimiTokenProvider) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if !account.IsKimiOAuth() {
		return "", errors.New("not a kimi oauth account")
	}

	cacheKey := KimiTokenCacheKey(account)
	if p.tokenCache != nil {
		if token, err := p.tokenCache.GetAccessToken(ctx, cacheKey); err == nil && strings.TrimSpace(token) != "" {
			return token, nil
		}
	}

	expiresAt := account.GetCredentialAsTime("expires_at")
	needsRefresh := expiresAt == nil || time.Until(*expiresAt) <= kimiTokenRefreshSkew
	if needsRefresh && p.refreshAPI != nil && p.executor != nil {
		refreshCtx, cancel := context.WithTimeout(ctx, kimiRequestRefreshTimeout)
		defer cancel()
		result, err := p.refreshAPI.RefreshIfNeeded(refreshCtx, account, p.executor, kimiTokenRefreshSkew)
		if err != nil {
			if p.refreshPolicy.OnRefreshError == ProviderRefreshErrorReturn {
				return "", err
			}
			slog.Warn("kimi_token_refresh_failed", "account_id", account.ID, "error", err)
		} else if result != nil && result.LockHeld {
			if p.refreshPolicy.OnLockHeld == ProviderLockHeldWaitForCache && p.tokenCache != nil {
				time.Sleep(kimiLockWaitTime)
				if token, cacheErr := p.tokenCache.GetAccessToken(ctx, cacheKey); cacheErr == nil && strings.TrimSpace(token) != "" {
					return token, nil
				}
			}
		} else if result != nil && result.Account != nil {
			account = result.Account
			expiresAt = account.GetCredentialAsTime("expires_at")
		}
	}

	accessToken := strings.TrimSpace(account.GetKimiAccessToken())
	if accessToken == "" {
		return "", errors.New("kimi oauth access token is missing")
	}
	if expiresAt != nil && !time.Now().Before(*expiresAt) {
		return "", errors.New("kimi oauth access token is expired")
	}
	if p.tokenCache != nil {
		ttl := kimiTokenCacheSkew
		if expiresAt != nil {
			if remaining := time.Until(*expiresAt) - time.Minute; remaining > 0 && remaining < ttl {
				ttl = remaining
			}
		}
		if err := p.tokenCache.SetAccessToken(ctx, cacheKey, accessToken, ttl); err != nil {
			slog.Warn("kimi_token_cache_set_failed", "account_id", account.ID, "error", err)
		}
	}
	return accessToken, nil
}

func KimiTokenCacheKey(account *Account) string {
	if account == nil {
		return "kimi:account:0"
	}
	return "kimi:account:" + strconv.FormatInt(account.ID, 10)
}
