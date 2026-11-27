//go:build unit

package service

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexRequestCompatibilityPreservesAccountSettings(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			credKeyCodexRequestCompatEnabled:   true,
			credKeyCodexRequestCompatUserAgent: "codex_exec/test",
			credKeyHeaderOverrideEnabled:       true,
			credKeyHeaderOverrides: map[string]any{
				"x-codex-installation-id": "existing-installation",
				"x-custom":                "preserved",
			},
		},
	}
	headers := http.Header{
		"USER-agent":    {"client"},
		"user-agent":    {"duplicate"},
		"Session_id":    {"request-session"},
		"Version":       {"request-version"},
		"Originator":    {"request-originator"},
		"Authorization": {"Bearer upstream-key"},
	}
	account.ApplyHeaderOverrides(context.Background(), headers)
	require.Equal(t, "codex_exec/test", headers.Get("User-Agent"))
	require.NotContains(t, headers, "USER-agent")
	require.NotContains(t, headers, "user-agent")
	require.Equal(t, []string{"existing-installation"}, headerValuesFold(headers, "x-codex-installation-id"))
	require.Equal(t, []string{"preserved"}, headerValuesFold(headers, "x-custom"))
	require.Equal(t, "request-session", headers.Get("Session_id"))
	require.Equal(t, "request-version", headers.Get("Version"))
	require.Equal(t, "request-originator", headers.Get("Originator"))
	require.Equal(t, "Bearer upstream-key", headers.Get("Authorization"))
	require.NotContains(t, headers, "X-Codex-Window-Id")
	require.NotContains(t, headers, "X-Codex-Turn-State")

	account.Credentials[credKeyCodexRequestCompatEnabled] = false
	headers = http.Header{"User-Agent": {"client"}}
	account.ApplyHeaderOverrides(context.Background(), headers)
	require.Equal(t, "client", headers.Get("User-Agent"))
	require.Equal(t, []string{"existing-installation"}, headerValuesFold(headers, "x-codex-installation-id"))
	require.Equal(t, "codex_exec/test", account.Credentials[credKeyCodexRequestCompatUserAgent])
}

func TestCodexRequestCompatibilityExplicitHeadersTakePriority(t *testing.T) {
	for _, tt := range []struct {
		name     string
		override string
		want     string
	}{
		{"literal", "existing-agent", "existing-agent"},
		{"blank placeholder", "", "codex_exec/test"},
		{"unresolved variable removes header", "{{user_id}}", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Platform: PlatformOpenAI,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					credKeyCodexRequestCompatEnabled:   true,
					credKeyCodexRequestCompatUserAgent: "codex_exec/test",
					credKeyHeaderOverrideEnabled:       true,
					credKeyHeaderOverrides:             map[string]any{"User-Agent": tt.override},
				},
			}
			headers := make(http.Header)
			account.ApplyHeaderOverrides(context.Background(), headers)
			require.Equal(t, tt.want, headers.Get("User-Agent"))
		})
	}
}

func TestCodexRequestCompatibilityEligibility(t *testing.T) {
	for _, tt := range []struct {
		platform string
		kind     string
		enabled  any
		ua       string
		want     string
	}{
		{PlatformOpenAI, AccountTypeAPIKey, true, "codex_exec/test", "codex_exec/test"},
		{PlatformOpenAI, AccountTypeAPIKey, false, "codex_exec/test", "client"},
		{PlatformOpenAI, AccountTypeAPIKey, "true", "codex_exec/test", "client"},
		{PlatformOpenAI, AccountTypeAPIKey, true, "bad\r\nheader", "client"},
		{PlatformOpenAI, AccountTypeAPIKey, true, "agent/{{user_id}}", "client"},
		{PlatformOpenAI, AccountTypeAPIKey, true, "", "client"},
		{PlatformOpenAI, AccountTypeOAuth, true, "codex_exec/test", "client"},
		{PlatformAnthropic, AccountTypeAPIKey, true, "codex_exec/test", "client"},
	} {
		account := &Account{
			Platform: tt.platform, Type: tt.kind,
			Credentials: map[string]any{
				credKeyCodexRequestCompatEnabled:   tt.enabled,
				credKeyCodexRequestCompatUserAgent: tt.ua,
			},
		}
		headers := http.Header{"User-Agent": {"client"}}
		account.ApplyHeaderOverrides(context.Background(), headers)
		require.Equal(t, tt.want, headers.Get("User-Agent"))
	}
	var account *Account
	account.ApplyHeaderOverrides(context.Background(), nil)
	headers := http.Header{"User-Agent": {"client"}}
	account.ApplyHeaderOverrides(context.Background(), headers)
	require.Equal(t, "client", headers.Get("User-Agent"))
}

func TestNormalizeCodexRequestCompatibility(t *testing.T) {
	for _, tt := range []struct {
		name  string
		creds map[string]any
		valid bool
	}{
		{"legacy", map[string]any{"api_key": "test"}, true},
		{"enabled", map[string]any{credKeyCodexRequestCompatEnabled: true, credKeyCodexRequestCompatUserAgent: " codex_exec/test "}, true},
		{"disabled retains UA", map[string]any{credKeyCodexRequestCompatEnabled: false, credKeyCodexRequestCompatUserAgent: "codex_exec/test"}, true},
		{"disabled empty", map[string]any{credKeyCodexRequestCompatEnabled: false}, true},
		{"enabled missing UA", map[string]any{credKeyCodexRequestCompatEnabled: true}, false},
		{"enabled empty UA", map[string]any{credKeyCodexRequestCompatEnabled: true, credKeyCodexRequestCompatUserAgent: "  "}, false},
		{"invalid flag", map[string]any{credKeyCodexRequestCompatEnabled: "true"}, false},
		{"null flag", map[string]any{credKeyCodexRequestCompatEnabled: nil}, false},
		{"invalid UA type", map[string]any{credKeyCodexRequestCompatUserAgent: 7}, false},
		{"null UA", map[string]any{credKeyCodexRequestCompatUserAgent: nil}, false},
		{"header injection", map[string]any{credKeyCodexRequestCompatUserAgent: "agent\r\nX-Key: secret"}, false},
		{"template not literal", map[string]any{credKeyCodexRequestCompatEnabled: true, credKeyCodexRequestCompatUserAgent: "agent/{{user_id}}"}, false},
		{"oversized", map[string]any{credKeyCodexRequestCompatUserAgent: strings.Repeat("a", maxHeaderOverrideValueLength+1)}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := NormalizeHeaderOverrideCredentials(tt.creds)
			if tt.valid {
				require.NoError(t, err)
				if ua, ok := tt.creds[credKeyCodexRequestCompatUserAgent].(string); ok {
					require.Equal(t, strings.TrimSpace(ua), ua)
				}
			} else {
				require.Error(t, err)
			}
		})
	}
	legacy := map[string]any{"api_key": "test"}
	require.NoError(t, NormalizeHeaderOverrideCredentials(legacy))
	require.Equal(t, map[string]any{"api_key": "test"}, legacy)
}

func TestBulkUpdateAccountsCodexRequestCompatibility(t *testing.T) {
	credentials := func(enabled bool, userAgent string) map[string]any {
		return map[string]any{
			credKeyCodexRequestCompatEnabled:   enabled,
			credKeyCodexRequestCompatUserAgent: userAgent,
			"api_key":                          "existing-key",
		}
	}
	for _, tt := range []struct {
		name      string
		accounts  []map[string]any
		patch     map[string]any
		wantPatch map[string]any
		invalid   bool
	}{
		{
			name:      "enable preserves each existing user agent",
			accounts:  []map[string]any{credentials(false, "agent/one"), credentials(false, "agent/two")},
			patch:     map[string]any{credKeyCodexRequestCompatEnabled: true},
			wantPatch: map[string]any{credKeyCodexRequestCompatEnabled: true},
		},
		{
			name:     "clear rejects an enabled first account",
			accounts: []map[string]any{credentials(true, "agent/one"), credentials(false, "agent/two")},
			patch:    map[string]any{credKeyCodexRequestCompatUserAgent: ""},
			invalid:  true,
		},
		{
			name:     "clear rejects an enabled later account before any write",
			accounts: []map[string]any{credentials(false, "agent/one"), credentials(true, "agent/two")},
			patch:    map[string]any{credKeyCodexRequestCompatUserAgent: "  "},
			invalid:  true,
		},
		{
			name:     "enable rejects a later account missing its user agent",
			accounts: []map[string]any{credentials(false, "agent/one"), {credKeyCodexRequestCompatEnabled: false}},
			patch:    map[string]any{credKeyCodexRequestCompatEnabled: true},
			invalid:  true,
		},
		{
			name:      "disable allows clearing user agents",
			accounts:  []map[string]any{credentials(true, "agent/one"), credentials(true, "agent/two")},
			patch:     map[string]any{credKeyCodexRequestCompatEnabled: false, credKeyCodexRequestCompatUserAgent: ""},
			wantPatch: map[string]any{credKeyCodexRequestCompatEnabled: false, credKeyCodexRequestCompatUserAgent: ""},
		},
		{
			name:      "disable preserves each existing user agent",
			accounts:  []map[string]any{credentials(true, "agent/one"), credentials(true, "agent/two")},
			patch:     map[string]any{credKeyCodexRequestCompatEnabled: false},
			wantPatch: map[string]any{credKeyCodexRequestCompatEnabled: false},
		},
		{
			name:      "user agent patch preserves each existing enabled state",
			accounts:  []map[string]any{credentials(true, "agent/one"), credentials(false, "agent/two")},
			patch:     map[string]any{credKeyCodexRequestCompatUserAgent: "  agent/new  "},
			wantPatch: map[string]any{credKeyCodexRequestCompatUserAgent: "agent/new"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &upstreamBillingProbeAccountRepo{accounts: make(map[int64]*Account)}
			originals := make(map[int64]map[string]any)
			accountIDs := make([]int64, 0, len(tt.accounts))
			for i, existing := range tt.accounts {
				id := int64(i + 1)
				accountIDs = append(accountIDs, id)
				repo.accounts[id] = &Account{
					ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
					Credentials: mergeMap(nil, existing),
				}
				originals[id] = mergeMap(nil, existing)
			}

			result, err := (&adminServiceImpl{accountRepo: repo}).BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
				AccountIDs: accountIDs, Credentials: tt.patch,
			})
			if tt.invalid {
				require.Error(t, err)
				require.Contains(t, err.Error(), "codex_request_compat_user_agent")
				require.Nil(t, result)
				require.Empty(t, repo.bulkUpdates)
			} else {
				require.NoError(t, err)
				require.Equal(t, len(accountIDs), result.Success)
				require.Len(t, repo.bulkUpdates, 1)
				require.Equal(t, tt.wantPatch, repo.bulkUpdates[0].Credentials)
			}
			for id, original := range originals {
				require.Equal(t, original, repo.accounts[id].Credentials, "validation mutated account %d", id)
			}
		})
	}
}

func TestCodexRequestCompatibilityUpdatesBillingProbeIdentity(t *testing.T) {
	for _, tt := range []struct {
		name  string
		patch map[string]any
	}{
		{"enabled state", map[string]any{credKeyCodexRequestCompatEnabled: false}},
		{"user agent", map[string]any{credKeyCodexRequestCompatUserAgent: "agent/new"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Credentials: map[string]any{
					credKeyCodexRequestCompatEnabled:   true,
					credKeyCodexRequestCompatUserAgent: "agent/old",
				},
				Extra: map[string]any{UpstreamBillingProbeExtraKey: map[string]any{"status": "ok"}},
			}
			before := upstreamBillingProbeIdentity(account)
			repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
			result, err := (&adminServiceImpl{accountRepo: repo}).BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
				AccountIDs: []int64{account.ID}, Credentials: tt.patch,
			})
			require.NoError(t, err)
			require.Equal(t, 1, result.Success)
			require.Len(t, repo.bulkUpdates, 1)
			require.Contains(t, repo.bulkUpdates[0].Extra, UpstreamBillingProbeExtraKey)
			require.Nil(t, repo.bulkUpdates[0].Extra[UpstreamBillingProbeExtraKey])
			require.Equal(t, before, upstreamBillingProbeIdentity(account))

			updated := *account
			updated.Credentials = mergeMap(account.Credentials, repo.bulkUpdates[0].Credentials)
			require.NotEqual(t, before, upstreamBillingProbeIdentity(&updated))
		})
	}
}
