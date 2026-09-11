package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
)

type kimiOAuthClient struct{}

func NewKimiOAuthClient() service.KimiOAuthClient {
	return &kimiOAuthClient{}
}

type kimiOAuthJSON map[string]any

func (c *kimiOAuthClient) RequestDeviceAuthorization(
	ctx context.Context,
	oauthHost, proxyURL string,
	headers map[string]string,
) (*service.KimiDeviceAuthorization, error) {
	host, err := validateKimiOAuthHost(oauthHost)
	if err != nil {
		return nil, err
	}
	status, data, err := c.postForm(ctx, host+"/api/oauth/device_authorization", url.Values{
		"client_id": {service.KimiCodeOAuthClientID},
	}, proxyURL, headers)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, kimiOAuthStatusError("KIMI_OAUTH_DEVICE_AUTH_FAILED", "device authorization failed", status, data)
	}
	userCode := kimiJSONString(data, "user_code")
	deviceCode := kimiJSONString(data, "device_code")
	completeURI := kimiJSONString(data, "verification_uri_complete")
	if userCode == "" || deviceCode == "" || completeURI == "" {
		return nil, infraerrors.New(http.StatusBadGateway, "KIMI_OAUTH_DEVICE_AUTH_INVALID", "device authorization response missing required fields")
	}
	expiresIn := kimiJSONInt(data, "expires_in")
	interval := kimiJSONInt(data, "interval")
	if interval <= 0 {
		interval = 5
	}
	return &service.KimiDeviceAuthorization{
		UserCode:                userCode,
		DeviceCode:              deviceCode,
		VerificationURI:         kimiJSONString(data, "verification_uri"),
		VerificationURIComplete: completeURI,
		ExpiresIn:               expiresIn,
		Interval:                interval,
	}, nil
}

func (c *kimiOAuthClient) PollDeviceToken(
	ctx context.Context,
	oauthHost, deviceCode, proxyURL string,
	headers map[string]string,
) (*service.KimiDevicePollResult, error) {
	host, err := validateKimiOAuthHost(oauthHost)
	if err != nil {
		return nil, err
	}
	status, data, err := c.postForm(ctx, host+"/api/oauth/token", url.Values{
		"client_id":   {service.KimiCodeOAuthClientID},
		"device_code": {strings.TrimSpace(deviceCode)},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}, proxyURL, headers)
	if err != nil {
		return nil, err
	}
	if status == http.StatusOK && kimiJSONString(data, "access_token") != "" {
		token, err := kimiTokenFromResponse(data)
		if err != nil {
			return nil, err
		}
		return &service.KimiDevicePollResult{Status: service.KimiDevicePollSuccess, TokenInfo: token}, nil
	}
	if status >= 500 {
		return nil, kimiOAuthStatusError("KIMI_OAUTH_POLL_FAILED", "device token polling failed", status, data)
	}
	errorCode := kimiJSONString(data, "error")
	if errorCode == "" {
		errorCode = "unknown_error"
	}
	description := kimiJSONString(data, "error_description")
	switch errorCode {
	case "authorization_pending", "slow_down":
		return &service.KimiDevicePollResult{
			Status:      service.KimiDevicePollPending,
			ErrorCode:   errorCode,
			Description: description,
		}, nil
	case "expired_token":
		return &service.KimiDevicePollResult{Status: service.KimiDevicePollExpired, ErrorCode: errorCode}, nil
	case "access_denied":
		return &service.KimiDevicePollResult{
			Status:      service.KimiDevicePollDenied,
			ErrorCode:   errorCode,
			Description: description,
		}, nil
	default:
		return nil, kimiOAuthStatusError("KIMI_OAUTH_POLL_FAILED", "device token polling failed", status, data)
	}
}

func (c *kimiOAuthClient) RefreshToken(
	ctx context.Context,
	oauthHost, refreshToken, proxyURL string,
	headers map[string]string,
) (*service.KimiTokenInfo, error) {
	host, err := validateKimiOAuthHost(oauthHost)
	if err != nil {
		return nil, err
	}
	status, data, err := c.postForm(ctx, host+"/api/oauth/token", url.Values{
		"client_id":     {service.KimiCodeOAuthClientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {strings.TrimSpace(refreshToken)},
	}, proxyURL, headers)
	if err != nil {
		return nil, err
	}
	if status == http.StatusOK && kimiJSONString(data, "access_token") != "" {
		return kimiTokenFromResponse(data)
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden || kimiJSONString(data, "error") == "invalid_grant" {
		return nil, infraerrors.New(http.StatusUnauthorized, "KIMI_OAUTH_UNAUTHORIZED", kimiOAuthErrorDetail(data, "token refresh unauthorized"))
	}
	return nil, kimiOAuthStatusError("KIMI_OAUTH_REFRESH_FAILED", "token refresh failed", status, data)
}

func (c *kimiOAuthClient) FetchUserInfo(
	ctx context.Context,
	baseURL, accessToken, proxyURL string,
	headers map[string]string,
) (*service.KimiUserInfo, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/me"
	client, err := getSharedReqClient(reqClientOptions{ProxyURL: proxyURL, Timeout: 15 * time.Second})
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "KIMI_OAUTH_CLIENT_INIT_FAILED", "create HTTP client: %v", err)
	}
	req := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	for key, value := range headers {
		req.SetHeader(key, value)
	}
	resp, err := req.Get(endpoint)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "KIMI_OAUTH_USERINFO_FAILED", "request failed: %v", err)
	}
	data := parseKimiJSON(resp.Bytes())
	if resp.StatusCode != http.StatusOK {
		return nil, kimiOAuthStatusError("KIMI_OAUTH_USERINFO_FAILED", "userinfo request failed", resp.StatusCode, data)
	}
	info := &service.KimiUserInfo{
		UserID:   firstNonEmpty(kimiJSONString(data, "user_id"), kimiJSONString(data, "userId")),
		Nickname: firstNonEmpty(kimiJSONString(data, "nickname"), kimiJSONString(data, "username")),
		Email:    kimiJSONString(data, "email"),
	}
	if info.UserID == "" && info.Nickname == "" && info.Email == "" {
		return nil, infraerrors.New(http.StatusBadGateway, "KIMI_OAUTH_USERINFO_INVALID", "userinfo response missing profile fields")
	}
	return info, nil
}

func (c *kimiOAuthClient) postForm(
	ctx context.Context,
	endpoint string,
	form url.Values,
	proxyURL string,
	headers map[string]string,
) (int, kimiOAuthJSON, error) {
	client, err := getSharedReqClient(reqClientOptions{ProxyURL: proxyURL, Timeout: 30 * time.Second})
	if err != nil {
		return 0, nil, infraerrors.Newf(http.StatusBadGateway, "KIMI_OAUTH_CLIENT_INIT_FAILED", "create HTTP client: %v", err)
	}
	req := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormDataFromValues(form)
	for key, value := range headers {
		req.SetHeader(key, value)
	}
	resp, err := req.Post(endpoint)
	if err != nil {
		return 0, nil, infraerrors.Newf(http.StatusBadGateway, "KIMI_OAUTH_REQUEST_FAILED", "request failed: %v", err)
	}
	return resp.StatusCode, parseKimiJSON(resp.Bytes()), nil
}

func validateKimiOAuthHost(oauthHost string) (string, error) {
	host := strings.TrimRight(strings.TrimSpace(oauthHost), "/")
	switch host {
	case service.DefaultKimiOAuthHostCN, service.DefaultKimiOAuthHostGlobal:
		return host, nil
	default:
		return "", infraerrors.New(http.StatusBadRequest, "KIMI_OAUTH_UNSUPPORTED_HOST", "unsupported kimi oauth host")
	}
}

func kimiTokenFromResponse(data kimiOAuthJSON) (*service.KimiTokenInfo, error) {
	accessToken := kimiJSONString(data, "access_token")
	refreshToken := kimiJSONString(data, "refresh_token")
	expiresIn := int64(kimiJSONInt(data, "expires_in"))
	if accessToken == "" || refreshToken == "" || expiresIn <= 0 {
		return nil, infraerrors.New(http.StatusBadGateway, "KIMI_OAUTH_INVALID_TOKEN_RESPONSE", "oauth token response missing required fields")
	}
	tokenType := kimiJSONString(data, "token_type")
	if tokenType == "" {
		tokenType = "Bearer"
	}
	return &service.KimiTokenInfo{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    tokenType,
		ExpiresIn:    expiresIn,
		ExpiresAt:    time.Now().Unix() + expiresIn,
		Scope:        kimiJSONString(data, "scope"),
		ClientID:     service.KimiCodeOAuthClientID,
	}, nil
}

func parseKimiJSON(raw []byte) kimiOAuthJSON {
	data := kimiOAuthJSON{}
	if len(raw) == 0 {
		return data
	}
	_ = json.Unmarshal(raw, &data)
	return data
}

func kimiJSONString(data kimiOAuthJSON, key string) string {
	if data == nil {
		return ""
	}
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func kimiJSONInt(data kimiOAuthJSON, key string) int {
	if data == nil {
		return 0
	}
	value, ok := data[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case json.Number:
		n, _ := typed.Int64()
		return int(n)
	case int:
		return typed
	case int64:
		return int(typed)
	case string:
		n := 0
		_, _ = fmt.Sscanf(strings.TrimSpace(typed), "%d", &n)
		return n
	default:
		return 0
	}
}

func kimiOAuthErrorDetail(data kimiOAuthJSON, fallback string) string {
	if detail := kimiJSONString(data, "error_description"); detail != "" {
		return detail
	}
	if detail := kimiJSONString(data, "message"); detail != "" {
		return detail
	}
	if detail := kimiJSONString(data, "error"); detail != "" {
		return detail
	}
	if fallback != "" {
		return fallback
	}
	return "kimi oauth request failed"
}

func kimiOAuthStatusError(code, message string, status int, data kimiOAuthJSON) error {
	detail := kimiOAuthErrorDetail(data, message)
	return infraerrors.Newf(http.StatusBadGateway, code, "%s (HTTP %d): %s", message, status, logredact.RedactText(detail))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
