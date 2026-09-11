package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestValidateKimiOAuthHost(t *testing.T) {
	t.Parallel()
	host, err := validateKimiOAuthHost("https://auth.kimi.com/")
	require.NoError(t, err)
	require.Equal(t, service.DefaultKimiOAuthHostCN, host)
	_, err = validateKimiOAuthHost("https://evil.example")
	require.Error(t, err)
}

func TestKimiTokenFromResponse(t *testing.T) {
	t.Parallel()
	token, err := kimiTokenFromResponse(kimiOAuthJSON{
		"access_token":  "at",
		"refresh_token": "rt",
		"expires_in":    float64(3600),
		"token_type":    "Bearer",
		"scope":         "openid",
	})
	require.NoError(t, err)
	require.Equal(t, "at", token.AccessToken)
	require.Equal(t, "rt", token.RefreshToken)
	require.Equal(t, int64(3600), token.ExpiresIn)

	_, err = kimiTokenFromResponse(kimiOAuthJSON{"access_token": "at", "refresh_token": "rt"})
	require.Error(t, err)

	client := NewKimiOAuthClient()
	_, err = client.RequestDeviceAuthorization(context.Background(), "https://evil.example", "", nil)
	require.Error(t, err)
}
