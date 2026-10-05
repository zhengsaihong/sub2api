package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSummarizeGroupQuotaIncludesCooldownAndExcludesUnavailableAccounts(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	cooldownUntil := now.Add(time.Hour)
	expired := now.Add(-time.Minute)

	accounts := []Account{
		{Status: StatusActive, Schedulable: true, Extra: map[string]any{
			"codex_5h_used_percent": 20,
			"codex_5h_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
			"codex_7d_used_percent": 40,
			"codex_7d_reset_at":     now.Add(24 * time.Hour).Format(time.RFC3339),
		}},
		{Status: StatusActive, Schedulable: true, RateLimitResetAt: &cooldownUntil, Extra: map[string]any{
			"codex_5h_used_percent": 50,
			"codex_5h_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
			"codex_7d_used_percent": 10,
			"codex_7d_reset_at":     now.Add(24 * time.Hour).Format(time.RFC3339),
		}},
		{Status: StatusError, Schedulable: true, Extra: map[string]any{"codex_5h_used_percent": 0}},
		{Status: StatusDisabled, Schedulable: true, Extra: map[string]any{"codex_5h_used_percent": 0}},
		{Status: StatusActive, Schedulable: false, Extra: map[string]any{"codex_5h_used_percent": 0}},
		{Status: StatusActive, Schedulable: true, AutoPauseOnExpired: true, ExpiresAt: &expired, Extra: map[string]any{"codex_5h_used_percent": 0}},
	}

	got := summarizeGroupQuota(accounts, now)

	require.Equal(t, 2, got.AccountCount)
	require.Equal(t, 1, got.CooldownAccountCount)
	require.NotNil(t, got.Remaining5hPercent)
	require.NotNil(t, got.Remaining7dPercent)
	require.InDelta(t, 65, *got.Remaining5hPercent, 0.001)
	require.InDelta(t, 75, *got.Remaining7dPercent, 0.001)
}

func TestSummarizeGroupQuotaLeavesUnknownWindowsUnset(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	got := summarizeGroupQuota([]Account{{Status: StatusActive, Schedulable: true}}, now)

	require.Equal(t, 1, got.AccountCount)
	require.Nil(t, got.Remaining5hPercent)
	require.Nil(t, got.Remaining7dPercent)
}
