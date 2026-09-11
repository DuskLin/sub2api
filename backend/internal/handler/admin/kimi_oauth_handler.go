package admin

import (
	"strconv"
	"strings"

	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type KimiOAuthHandler struct {
	kimiOAuthService *service.KimiOAuthService
	adminService     service.AdminService
}

func NewKimiOAuthHandler(kimiOAuthService *service.KimiOAuthService, adminService service.AdminService) *KimiOAuthHandler {
	return &KimiOAuthHandler{
		kimiOAuthService: kimiOAuthService,
		adminService:     adminService,
	}
}

type KimiDeviceAuthorizationRequest struct {
	Region  string `json:"region"`
	ProxyID *int64 `json:"proxy_id"`
}

func (h *KimiOAuthHandler) StartDeviceAuthorization(c *gin.Context) {
	if h == nil || h.kimiOAuthService == nil {
		response.ErrorFrom(c, infraerrors.New(http.StatusInternalServerError, "KIMI_OAUTH_NOT_CONFIGURED", "kimi oauth service is not configured"))
		return
	}
	var req KimiDeviceAuthorizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = KimiDeviceAuthorizationRequest{}
	}
	result, err := h.kimiOAuthService.StartDeviceAuthorization(c.Request.Context(), req.Region, req.ProxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

type KimiDevicePollRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

func (h *KimiOAuthHandler) PollDeviceAuthorization(c *gin.Context) {
	if h == nil || h.kimiOAuthService == nil {
		response.ErrorFrom(c, infraerrors.New(http.StatusInternalServerError, "KIMI_OAUTH_NOT_CONFIGURED", "kimi oauth service is not configured"))
		return
	}
	var req KimiDevicePollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.kimiOAuthService.PollDeviceAuthorization(c.Request.Context(), req.SessionID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

type KimiRefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
	RT           string `json:"rt"`
	Region       string `json:"region"`
	OAuthHost    string `json:"oauth_host"`
	DeviceID     string `json:"device_id"`
	ProxyID      *int64 `json:"proxy_id"`
}

func (h *KimiOAuthHandler) RefreshToken(c *gin.Context) {
	if h == nil || h.kimiOAuthService == nil {
		response.ErrorFrom(c, infraerrors.New(http.StatusInternalServerError, "KIMI_OAUTH_NOT_CONFIGURED", "kimi oauth service is not configured"))
		return
	}
	var req KimiRefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		refreshToken = strings.TrimSpace(req.RT)
	}
	if refreshToken == "" {
		response.BadRequest(c, "refresh_token is required")
		return
	}
	var proxyURL string
	if req.ProxyID != nil {
		proxy, err := h.adminService.GetProxy(c.Request.Context(), *req.ProxyID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if proxy == nil {
			response.BadRequest(c, "KIMI_OAUTH_PROXY_NOT_FOUND: proxy not found")
			return
		}
		proxyURL = proxy.URL()
	}
	oauthHost := strings.TrimSpace(req.OAuthHost)
	if oauthHost == "" {
		oauthHost, _, _ = service.KimiRegionEndpoints(req.Region)
	}
	tokenInfo, err := h.kimiOAuthService.RefreshToken(c.Request.Context(), refreshToken, proxyURL, oauthHost, req.DeviceID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

func (h *KimiOAuthHandler) RefreshAccountToken(c *gin.Context) {
	if h == nil || h.kimiOAuthService == nil {
		response.ErrorFrom(c, infraerrors.New(http.StatusInternalServerError, "KIMI_OAUTH_NOT_CONFIGURED", "kimi oauth service is not configured"))
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !account.IsKimiOAuth() {
		response.BadRequest(c, "Account is not a Kimi OAuth account")
		return
	}
	tokenInfo, err := h.kimiOAuthService.RefreshAccountToken(c.Request.Context(), account)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	credentials := service.MergeCredentials(account.Credentials, h.kimiOAuthService.BuildAccountCredentials(tokenInfo))
	updated, err := h.adminService.UpdateAccount(c.Request.Context(), account.ID, &service.UpdateAccountInput{
		Credentials: credentials,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(updated))
}
