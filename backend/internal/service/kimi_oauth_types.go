package service

import "time"

// KimiDeviceAuthorization is the RFC 8628 device-authorization response.
type KimiDeviceAuthorization struct {
	UserCode                string `json:"user_code"`
	DeviceCode              string `json:"device_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// KimiDeviceAuthResult is returned to the admin UI after starting a device flow.
// Device code is never included — the browser only sees the user-facing fields.
type KimiDeviceAuthResult struct {
	SessionID               string `json:"session_id"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
	ExpiresAt               int64  `json:"expires_at"`
	Region                  string `json:"region"`
	OAuthHost               string `json:"oauth_host"`
}

const (
	KimiDevicePollPending = "pending"
	KimiDevicePollSuccess = "success"
	KimiDevicePollExpired = "expired"
	KimiDevicePollDenied  = "denied"
)

// KimiDevicePollResult is the admin poll response for a pending device flow.
type KimiDevicePollResult struct {
	Status      string         `json:"status"`
	Interval    int            `json:"interval,omitempty"`
	ErrorCode   string         `json:"error_code,omitempty"`
	Description string         `json:"description,omitempty"`
	TokenInfo   *KimiTokenInfo `json:"token_info,omitempty"`
}

// KimiTokenInfo is the persisted OAuth token bundle plus account profile.
type KimiTokenInfo struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
	Scope        string `json:"scope,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
	Region       string `json:"region,omitempty"`
	OAuthHost    string `json:"oauth_host,omitempty"`
	DeviceID     string `json:"device_id,omitempty"`
	Email        string `json:"email,omitempty"`
	Nickname     string `json:"nickname,omitempty"`
	UserID       string `json:"user_id,omitempty"`
}

// KimiUserInfo is a subset of GET {base}/me used for account naming.
type KimiUserInfo struct {
	UserID   string
	Nickname string
	Email    string
}

type kimiDeviceSession struct {
	DeviceCode string
	Interval   int
	OAuthHost  string
	Region     string
	ProxyID    *int64
	DeviceID   string
	ExpiresAt  time.Time
}
