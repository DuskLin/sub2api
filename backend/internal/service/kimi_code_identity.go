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
