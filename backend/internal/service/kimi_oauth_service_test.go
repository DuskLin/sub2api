package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type kimiOAuthClientStub struct {
	auth     *KimiDeviceAuthorization
	authErr  error
	poll     *KimiDevicePollResult
	pollErr  error
	token    *KimiTokenInfo
	tokenErr error
	user     *KimiUserInfo
	userErr  error
}

func (c *kimiOAuthClientStub) RequestDeviceAuthorization(context.Context, string, string, map[string]string) (*KimiDeviceAuthorization, error) {
	return c.auth, c.authErr
}

func (c *kimiOAuthClientStub) PollDeviceToken(context.Context, string, string, string, map[string]string) (*KimiDevicePollResult, error) {
	return c.poll, c.pollErr
}

func (c *kimiOAuthClientStub) RefreshToken(context.Context, string, string, string, map[string]string) (*KimiTokenInfo, error) {
	return c.token, c.tokenErr
}

func (c *kimiOAuthClientStub) FetchUserInfo(context.Context, string, string, string, map[string]string) (*KimiUserInfo, error) {
	return c.user, c.userErr
}

func TestKimiOAuthServiceDeviceFlowSuccess(t *testing.T) {
	t.Parallel()
	client := &kimiOAuthClientStub{
		auth: &KimiDeviceAuthorization{
			UserCode:                "WDJB-MJHT",
			DeviceCode:              "device-code",
			VerificationURI:         "https://auth.kimi.com/device",
			VerificationURIComplete: "https://auth.kimi.com/device?user_code=WDJB-MJHT",
			ExpiresIn:               600,
			Interval:                5,
		},
		poll: &KimiDevicePollResult{
			Status: KimiDevicePollSuccess,
			TokenInfo: &KimiTokenInfo{
				AccessToken:  "access",
				RefreshToken: "refresh",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				ExpiresAt:    time.Now().Add(time.Hour).Unix(),
			},
		},
		user: &KimiUserInfo{UserID: "u_1", Nickname: "moon", Email: "user@example.com"},
	}
	svc := NewKimiOAuthService(nil, client)
	t.Cleanup(svc.Stop)

	started, err := svc.StartDeviceAuthorization(context.Background(), "global", nil)
	require.NoError(t, err)
	require.Equal(t, KimiRegionGlobal, started.Region)
	require.Equal(t, DefaultKimiOAuthHostGlobal, started.OAuthHost)
	require.Equal(t, "WDJB-MJHT", started.UserCode)
	require.NotEmpty(t, started.SessionID)

	result, err := svc.PollDeviceAuthorization(context.Background(), started.SessionID)
	require.NoError(t, err)
	require.Equal(t, KimiDevicePollSuccess, result.Status)
	require.Equal(t, "access", result.TokenInfo.AccessToken)
	require.Equal(t, "u_1", result.TokenInfo.UserID)
	require.Equal(t, "moon", result.TokenInfo.Nickname)
	require.Equal(t, KimiRegionGlobal, result.TokenInfo.Region)

	creds := svc.BuildAccountCredentials(result.TokenInfo)
	require.Equal(t, AccountModeCoding, creds["account_mode"])
	require.Equal(t, KimiRegionGlobal, creds["region"])
	require.Equal(t, "user@example.com", creds["email"])
	require.NotContains(t, creds, "api_protocol")
	require.NotContains(t, creds, "base_url")
	require.NotContains(t, creds, "api_base_urls")

	preserved := MergeCredentials(map[string]any{
		"api_protocol": APIProtocolAnthropic,
		"base_url":     "https://relay.example.com/coding",
		"api_base_urls": map[string]any{
			APIProtocolChatCompletions: "https://relay.example.com/v1",
		},
	}, creds)
	require.Equal(t, APIProtocolAnthropic, preserved["api_protocol"])
	require.Equal(t, "https://relay.example.com/coding", preserved["base_url"])
}

func TestKimiOAuthServicePollPendingSlowDown(t *testing.T) {
	t.Parallel()
	client := &kimiOAuthClientStub{
		auth: &KimiDeviceAuthorization{
			UserCode: "CODE", DeviceCode: "dc", VerificationURIComplete: "https://auth.kimi.com/device?user_code=CODE",
			ExpiresIn: 600, Interval: 5,
		},
		poll: &KimiDevicePollResult{Status: KimiDevicePollPending, ErrorCode: "slow_down"},
	}
	svc := NewKimiOAuthService(nil, client)
	t.Cleanup(svc.Stop)
	started, err := svc.StartDeviceAuthorization(context.Background(), "", nil)
	require.NoError(t, err)
	result, err := svc.PollDeviceAuthorization(context.Background(), started.SessionID)
	require.NoError(t, err)
	require.Equal(t, KimiDevicePollPending, result.Status)
	require.Equal(t, 10, result.Interval)
}

func TestKimiOAuthServiceRefreshPreservesOriginalRefreshToken(t *testing.T) {
	t.Parallel()
	client := &kimiOAuthClientStub{
		token: &KimiTokenInfo{AccessToken: "new-access", RefreshToken: "", TokenType: "Bearer", ExpiresIn: 3600, ExpiresAt: time.Now().Add(time.Hour).Unix()},
	}
	svc := NewKimiOAuthService(nil, client)
	t.Cleanup(svc.Stop)
	info, err := svc.RefreshToken(context.Background(), "old-refresh", "", DefaultKimiOAuthHostCN, "device-1")
	require.NoError(t, err)
	require.Equal(t, "new-access", info.AccessToken)
	require.Equal(t, "old-refresh", info.RefreshToken)
	require.Equal(t, KimiRegionMainlandCN, info.Region)
}

func TestKimiOAuthServiceRefreshAccountTokenRejectsNonKimi(t *testing.T) {
	t.Parallel()
	svc := NewKimiOAuthService(nil, &kimiOAuthClientStub{})
	t.Cleanup(svc.Stop)
	_, err := svc.RefreshAccountToken(context.Background(), &Account{Platform: PlatformGrok, Type: AccountTypeOAuth})
	require.Error(t, err)
	var apiErr *infraerrors.ApplicationError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, int32(http.StatusBadRequest), apiErr.Code)
}

func TestKimiTokenRefresherNeedsRefresh(t *testing.T) {
	t.Parallel()
	refresher := NewKimiTokenRefresher(nil)
	expired := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	account := &Account{
		Platform: PlatformKimi,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "at",
			"refresh_token": "rt",
			"expires_at":    expired,
		},
	}
	require.True(t, refresher.CanRefresh(account))
	require.True(t, refresher.NeedsRefresh(account, time.Minute))
}

func TestIsKimiOAuthAndRegionDefaults(t *testing.T) {
	t.Parallel()
	account := &Account{
		Platform: PlatformKimi,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"region":       "global",
			"account_mode": AccountModeCoding,
			"access_token": "at",
		},
	}
	require.True(t, account.IsKimiOAuth())
	require.Equal(t, KimiRegionGlobal, account.GetKimiRegion())
	require.Equal(t, DefaultKimiGlobalCodingBaseURL, account.GetOpenAIBaseURL())
	require.Equal(t, "at", account.GetOpenAIProtocolAPIKey())
	require.Equal(t, "at", account.GetCNAuthToken())
	require.Equal(t, PlatformKimi, account.GetCodingPlanProvider())
}

func TestKimiCodeIdentityHeaders(t *testing.T) {
	t.Parallel()
	headers := kimiCodeIdentityHeaders("device-123")
	require.Equal(t, KimiCodeCLIUserAgent, headers["User-Agent"])
	require.Equal(t, KimiCodeCLIPlatform, headers["X-Msh-Platform"])
	require.Equal(t, "device-123", headers["X-Msh-Device-Id"])
}
