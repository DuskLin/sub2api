package service

import (
	"net/http"
	"runtime"
	"strings"
)

// kimiCodeIdentityHeaders returns the X-Msh-* / User-Agent set required by
// Kimi Code OAuth and managed Coding Plan endpoints.
func kimiCodeIdentityHeaders(deviceID string) map[string]string {
	id := strings.TrimSpace(deviceID)
	if id == "" {
		id = "unknown"
	}
	return map[string]string{
		"User-Agent":         KimiCodeCLIUserAgent,
		"X-Msh-Platform":     KimiCodeCLIPlatform,
		"X-Msh-Version":      KimiCodeCLIVersion,
		"X-Msh-Device-Name":  "sub2api",
		"X-Msh-Device-Model": asciiHeaderValue(runtime.GOOS + " " + runtime.GOARCH),
		"X-Msh-Os-Version":   asciiHeaderValue(runtime.GOOS),
		"X-Msh-Device-Id":    asciiHeaderValue(id),
	}
}

func applyKimiCodeIdentityHeaders(headers http.Header, deviceID string) {
	if headers == nil {
		return
	}
	for key, value := range kimiCodeIdentityHeaders(deviceID) {
		headers.Set(key, value)
	}
}

func (a *Account) ApplyKimiCodeIdentityHeaders(headers http.Header) {
	if a == nil || !a.IsKimiOAuth() {
		return
	}
	applyKimiCodeIdentityHeaders(headers, a.GetKimiDeviceID())
}

func isKimiCodeSealedIdentityHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "user-agent",
		"x-msh-platform",
		"x-msh-version",
		"x-msh-device-name",
		"x-msh-device-model",
		"x-msh-os-version",
		"x-msh-device-id":
		return true
	default:
		return false
	}
}

// SealKimiOAuthUpstreamHeaders 在账号覆写之后重新盖上 Kimi Code CLI 身份。
// OAuth 出站必须始终使用官方 CLI UA / X-Msh-*，不能被 ForceCodexCLI、
// 客户端 UA 或 header_overrides 改掉。
func (a *Account) SealKimiOAuthUpstreamHeaders(headers http.Header) {
	a.ApplyKimiCodeIdentityHeaders(headers)
}

func asciiHeaderValue(value string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e {
			return -1
		}
		return r
	}, strings.TrimSpace(value))
	if cleaned == "" {
		return "unknown"
	}
	return cleaned
}
