package service

import (
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	credKeyCodexRequestCompatEnabled   = "codex_request_compat_enabled"
	credKeyCodexRequestCompatUserAgent = "codex_request_compat_user_agent"
)

// Compatibility owns only its User-Agent. Explicit account header overrides run
// afterwards, and disabling compatibility leaves existing account settings intact.
func (a *Account) applyCodexRequestCompatibility(h http.Header) {
	if a == nil || h == nil || a.Platform != PlatformOpenAI || a.Type != AccountTypeAPIKey {
		return
	}
	enabled, _ := a.Credentials[credKeyCodexRequestCompatEnabled].(bool)
	if !enabled {
		return
	}
	value, _ := a.Credentials[credKeyCodexRequestCompatUserAgent].(string)
	if strings.Contains(value, "{{") {
		return
	}
	_, value, err := normalizeHeaderOverrideEntry("user-agent", value)
	if err != nil || value == "" {
		return
	}
	for name := range h {
		if strings.EqualFold(name, "user-agent") {
			delete(h, name)
		}
	}
	h.Set("User-Agent", value)
}

func normalizeCodexRequestCompatibility(credentials map[string]any, requireComplete bool) error {
	enabled := false
	if raw, exists := credentials[credKeyCodexRequestCompatEnabled]; exists {
		value, ok := raw.(bool)
		if !ok {
			return infraerrors.New(http.StatusBadRequest, "INVALID_CODEX_REQUEST_COMPAT",
				"codex_request_compat_enabled must be a boolean")
		}
		enabled = value
	}
	var userAgent string
	if raw, exists := credentials[credKeyCodexRequestCompatUserAgent]; exists {
		value, ok := raw.(string)
		if !ok {
			return infraerrors.New(http.StatusBadRequest, "INVALID_CODEX_REQUEST_COMPAT",
				"codex_request_compat_user_agent must be a string")
		}
		if strings.Contains(value, "{{") {
			return infraerrors.New(http.StatusBadRequest, "INVALID_CODEX_REQUEST_COMPAT",
				"codex_request_compat_user_agent must be a literal value")
		}
		_, normalized, err := normalizeHeaderOverrideEntry("user-agent", value)
		if err != nil {
			return err
		}
		userAgent = normalized
		credentials[credKeyCodexRequestCompatUserAgent] = userAgent
	}
	if requireComplete && enabled && userAgent == "" {
		return infraerrors.New(http.StatusBadRequest, "INVALID_CODEX_REQUEST_COMPAT",
			"codex_request_compat_user_agent is required when Codex request compatibility is enabled")
	}
	return nil
}
