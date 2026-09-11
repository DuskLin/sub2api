package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const kimiDeviceSessionTTL = 15 * time.Minute

type KimiOAuthService struct {
	proxyRepo   ProxyRepository
	oauthClient KimiOAuthClient
	sessions    sync.Map
	stopCh      chan struct{}
	stopOnce    sync.Once
}

func NewKimiOAuthService(proxyRepo ProxyRepository, oauthClient KimiOAuthClient) *KimiOAuthService {
	svc := &KimiOAuthService{
		proxyRepo:   proxyRepo,
		oauthClient: oauthClient,
		stopCh:      make(chan struct{}),
	}
	go svc.reapExpiredSessions()
	return svc
}

func (s *KimiOAuthService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *KimiOAuthService) StartDeviceAuthorization(ctx context.Context, region string, proxyID *int64) (*KimiDeviceAuthResult, error) {
	if s == nil || s.oauthClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "KIMI_OAUTH_NOT_CONFIGURED", "kimi oauth client is not configured")
	}
	region = NormalizeKimiRegion(region)
	oauthHost, _, _ := KimiRegionEndpoints(region)
	proxyURL, err := s.proxyURL(ctx, proxyID)
	if err != nil {
		return nil, err
	}
	deviceID := newKimiDeviceID()
	headers := kimiCodeIdentityHeaders(deviceID)
	auth, err := s.oauthClient.RequestDeviceAuthorization(ctx, oauthHost, proxyURL, headers)
	if err != nil {
		return nil, err
	}
	expiresIn := auth.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = int(kimiDeviceSessionTTL.Seconds())
	}
	interval := auth.Interval
	if interval <= 0 {
		interval = 5
	}
	sessionID := newKimiSessionID()
	s.sessions.Store(sessionID, &kimiDeviceSession{
		DeviceCode: auth.DeviceCode,
		Interval:   interval,
		OAuthHost:  oauthHost,
		Region:     region,
		ProxyID:    cloneKimiProxyID(proxyID),
		DeviceID:   deviceID,
		ExpiresAt:  time.Now().Add(time.Duration(expiresIn) * time.Second),
	})
	return &KimiDeviceAuthResult{
		SessionID:               sessionID,
		UserCode:                auth.UserCode,
		VerificationURI:         auth.VerificationURI,
		VerificationURIComplete: auth.VerificationURIComplete,
		ExpiresIn:               expiresIn,
		Interval:                interval,
		ExpiresAt:               time.Now().Add(time.Duration(expiresIn) * time.Second).Unix(),
		Region:                  region,
		OAuthHost:               oauthHost,
	}, nil
}

func (s *KimiOAuthService) PollDeviceAuthorization(ctx context.Context, sessionID string) (*KimiDevicePollResult, error) {
	if s == nil || s.oauthClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "KIMI_OAUTH_NOT_CONFIGURED", "kimi oauth client is not configured")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "KIMI_OAUTH_SESSION_REQUIRED", "session_id is required")
	}
	raw, ok := s.sessions.Load(sessionID)
	if !ok {
		return nil, infraerrors.New(http.StatusBadRequest, "KIMI_OAUTH_SESSION_NOT_FOUND", "kimi oauth session not found or expired")
	}
	session, _ := raw.(*kimiDeviceSession)
	if session == nil || time.Now().After(session.ExpiresAt) {
		s.sessions.Delete(sessionID)
		return &KimiDevicePollResult{Status: KimiDevicePollExpired}, nil
	}
	proxyURL, err := s.proxyURL(ctx, session.ProxyID)
	if err != nil {
		return nil, err
	}
	headers := kimiCodeIdentityHeaders(session.DeviceID)
	result, err := s.oauthClient.PollDeviceToken(ctx, session.OAuthHost, session.DeviceCode, proxyURL, headers)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, infraerrors.New(http.StatusBadGateway, "KIMI_OAUTH_POLL_FAILED", "empty device poll result")
	}
	switch result.Status {
	case KimiDevicePollPending:
		interval := session.Interval
		if result.ErrorCode == "slow_down" {
			interval += 5
			session.Interval = interval
			s.sessions.Store(sessionID, session)
		}
		result.Interval = interval
		return result, nil
	case KimiDevicePollExpired, KimiDevicePollDenied:
		s.sessions.Delete(sessionID)
		return result, nil
	case KimiDevicePollSuccess:
		s.sessions.Delete(sessionID)
		if result.TokenInfo == nil {
			return nil, infraerrors.New(http.StatusBadGateway, "KIMI_OAUTH_INVALID_TOKEN_RESPONSE", "oauth token response missing required fields")
		}
		s.decorateTokenInfo(ctx, result.TokenInfo, session, proxyURL)
		return result, nil
	default:
		return result, nil
	}
}

func (s *KimiOAuthService) RefreshToken(ctx context.Context, refreshToken, proxyURL, oauthHost, deviceID string) (*KimiTokenInfo, error) {
	if s == nil || s.oauthClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "KIMI_OAUTH_NOT_CONFIGURED", "kimi oauth client is not configured")
	}
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "KIMI_OAUTH_NO_REFRESH_TOKEN", "refresh_token is required")
	}
	host, _, _ := KimiRegionEndpoints(regionForOAuthHost(oauthHost))
	if strings.TrimSpace(oauthHost) != "" {
		host = strings.TrimRight(strings.TrimSpace(oauthHost), "/")
	}
	headers := kimiCodeIdentityHeaders(deviceID)
	tokenInfo, err := s.oauthClient.RefreshToken(ctx, host, refreshToken, proxyURL, headers)
	if err != nil {
		return nil, err
	}
	if tokenInfo == nil {
		return nil, infraerrors.New(http.StatusBadGateway, "KIMI_OAUTH_INVALID_TOKEN_RESPONSE", "oauth token response missing required fields")
	}
	if strings.TrimSpace(tokenInfo.RefreshToken) == "" {
		tokenInfo.RefreshToken = refreshToken
	}
	tokenInfo.OAuthHost = host
	tokenInfo.Region = regionForOAuthHost(host)
	tokenInfo.DeviceID = strings.TrimSpace(deviceID)
	tokenInfo.ClientID = KimiCodeOAuthClientID
	return tokenInfo, nil
}

func (s *KimiOAuthService) RefreshAccountToken(ctx context.Context, account *Account) (*KimiTokenInfo, error) {
	if account == nil || !account.IsKimiOAuth() {
		return nil, infraerrors.New(http.StatusBadRequest, "KIMI_OAUTH_INVALID_ACCOUNT", "account is not a Kimi OAuth account")
	}
	proxyURL, err := s.proxyURL(ctx, account.ProxyID)
	if err != nil {
		return nil, err
	}
	tokenInfo, err := s.RefreshToken(ctx, account.GetKimiRefreshToken(), proxyURL, account.GetKimiOAuthHost(), account.GetKimiDeviceID())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(tokenInfo.Email) == "" {
		tokenInfo.Email = account.GetCredential("email")
	}
	if strings.TrimSpace(tokenInfo.Nickname) == "" {
		tokenInfo.Nickname = account.GetCredential("nickname")
	}
	if strings.TrimSpace(tokenInfo.UserID) == "" {
		tokenInfo.UserID = account.GetCredential("user_id")
	}
	if strings.TrimSpace(tokenInfo.DeviceID) == "" {
		tokenInfo.DeviceID = account.GetKimiDeviceID()
	}
	return tokenInfo, nil
}

func (s *KimiOAuthService) BuildAccountCredentials(tokenInfo *KimiTokenInfo) map[string]any {
	if tokenInfo == nil {
		return nil
	}
	region := NormalizeKimiRegion(tokenInfo.Region)
	oauthHost, codingBase, anthropicBase := KimiRegionEndpoints(region)
	if strings.TrimSpace(tokenInfo.OAuthHost) != "" {
		oauthHost = strings.TrimRight(strings.TrimSpace(tokenInfo.OAuthHost), "/")
	}
	expiresAt := time.Unix(tokenInfo.ExpiresAt, 0).UTC().Format(time.RFC3339)
	creds := map[string]any{
		"access_token":  tokenInfo.AccessToken,
		"refresh_token": tokenInfo.RefreshToken,
		"token_type":    tokenInfo.TokenType,
		"expires_at":    expiresAt,
		"expires_in":    tokenInfo.ExpiresIn,
		"client_id":     KimiCodeOAuthClientID,
		"account_mode":  AccountModeCoding,
		"api_protocol":  APIProtocolAdaptive,
		"region":        region,
		"oauth_host":    oauthHost,
		"base_url":      codingBase,
		"api_base_urls": map[string]any{
			APIProtocolChatCompletions: codingBase,
			APIProtocolAnthropic:       anthropicBase,
			APIProtocolResponses:       codingBase,
		},
	}
	if tokenInfo.Scope != "" {
		creds["scope"] = tokenInfo.Scope
	}
	if tokenInfo.DeviceID != "" {
		creds["device_id"] = tokenInfo.DeviceID
	}
	if tokenInfo.Email != "" {
		creds["email"] = tokenInfo.Email
	}
	if tokenInfo.Nickname != "" {
		creds["nickname"] = tokenInfo.Nickname
	}
	if tokenInfo.UserID != "" {
		creds["user_id"] = tokenInfo.UserID
	}
	return creds
}

func (s *KimiOAuthService) decorateTokenInfo(ctx context.Context, tokenInfo *KimiTokenInfo, session *kimiDeviceSession, proxyURL string) {
	if tokenInfo == nil || session == nil {
		return
	}
	tokenInfo.Region = session.Region
	tokenInfo.OAuthHost = session.OAuthHost
	tokenInfo.DeviceID = session.DeviceID
	tokenInfo.ClientID = KimiCodeOAuthClientID
	_, codingBase, _ := KimiRegionEndpoints(session.Region)
	info, err := s.oauthClient.FetchUserInfo(ctx, codingBase, tokenInfo.AccessToken, proxyURL, kimiCodeIdentityHeaders(session.DeviceID))
	if err != nil || info == nil {
		return
	}
	tokenInfo.UserID = info.UserID
	tokenInfo.Nickname = info.Nickname
	tokenInfo.Email = info.Email
}

func (s *KimiOAuthService) proxyURL(ctx context.Context, proxyID *int64) (string, error) {
	if s == nil || proxyID == nil || s.proxyRepo == nil {
		return "", nil
	}
	proxy, err := s.proxyRepo.GetByID(ctx, *proxyID)
	if err != nil {
		return "", infraerrors.New(http.StatusBadGateway, "KIMI_OAUTH_PROXY_NOT_AVAILABLE", "unable to load kimi oauth proxy")
	}
	if proxy == nil {
		return "", infraerrors.New(http.StatusBadRequest, "KIMI_OAUTH_PROXY_NOT_FOUND", "proxy not found")
	}
	return proxy.URL(), nil
}

func (s *KimiOAuthService) reapExpiredSessions() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			now := time.Now()
			s.sessions.Range(func(key, value any) bool {
				session, _ := value.(*kimiDeviceSession)
				if session == nil || now.After(session.ExpiresAt) {
					s.sessions.Delete(key)
				}
				return true
			})
		}
	}
}

func regionForOAuthHost(oauthHost string) string {
	host := strings.TrimRight(strings.TrimSpace(oauthHost), "/")
	if strings.EqualFold(host, DefaultKimiOAuthHostGlobal) {
		return KimiRegionGlobal
	}
	return KimiRegionMainlandCN
}

func cloneKimiProxyID(proxyID *int64) *int64 {
	if proxyID == nil {
		return nil
	}
	copied := *proxyID
	return &copied
}

func newKimiSessionID() string {
	return kimiRandomHex(16)
}

func newKimiDeviceID() string {
	return kimiRandomHex(16)
}

func kimiRandomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf)
}
