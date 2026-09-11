package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type kimiOAuthClientStub struct {
	auth *service.KimiDeviceAuthorization
}

func (c *kimiOAuthClientStub) RequestDeviceAuthorization(context.Context, string, string, map[string]string) (*service.KimiDeviceAuthorization, error) {
	return c.auth, nil
}

func (c *kimiOAuthClientStub) PollDeviceToken(context.Context, string, string, string, map[string]string) (*service.KimiDevicePollResult, error) {
	return &service.KimiDevicePollResult{Status: service.KimiDevicePollPending}, nil
}

func (c *kimiOAuthClientStub) RefreshToken(context.Context, string, string, string, map[string]string) (*service.KimiTokenInfo, error) {
	return nil, nil
}

func (c *kimiOAuthClientStub) FetchUserInfo(context.Context, string, string, string, map[string]string) (*service.KimiUserInfo, error) {
	return nil, nil
}

func TestKimiOAuthHandlerStartDeviceAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := &kimiOAuthClientStub{
		auth: &service.KimiDeviceAuthorization{
			UserCode:                "ABCD-EFGH",
			DeviceCode:              "dc",
			VerificationURIComplete: "https://auth.kimi.com/device?user_code=ABCD-EFGH",
			ExpiresIn:               600,
			Interval:                5,
		},
	}
	svc := service.NewKimiOAuthService(nil, client)
	t.Cleanup(svc.Stop)
	handler := NewKimiOAuthHandler(svc, nil)

	router := gin.New()
	router.POST("/oauth/device-authorization", handler.StartDeviceAuthorization)
	body, _ := json.Marshal(map[string]string{"region": "global"})
	req := httptest.NewRequest(http.MethodPost, "/oauth/device-authorization", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope struct {
		Data service.KimiDeviceAuthResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, "ABCD-EFGH", envelope.Data.UserCode)
	require.Equal(t, service.KimiRegionGlobal, envelope.Data.Region)
	require.NotEmpty(t, envelope.Data.SessionID)
}
