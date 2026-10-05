package service

import (
	"context"
	"time"
)

// GroupQuotaSummary is the aggregate remaining provider quota visible to one API-key group.
type GroupQuotaSummary struct {
	GroupID              int64    `json:"group_id"`
	AccountCount         int      `json:"account_count"`
	CooldownAccountCount int      `json:"cooldown_account_count"`
	Remaining5hPercent   *float64 `json:"remaining_5h_percent,omitempty"`
	Remaining7dPercent   *float64 `json:"remaining_7d_percent,omitempty"`
}

// GetGroupQuotaSummary reads the latest persisted account quota snapshots for a group.
func (s *GatewayService) GetGroupQuotaSummary(ctx context.Context, groupID int64) (GroupQuotaSummary, error) {
	if s == nil || s.accountRepo == nil {
		return GroupQuotaSummary{}, ErrAccountNotFound
	}
	accounts, err := s.accountRepo.ListByGroup(ctx, groupID)
	if err != nil {
		return GroupQuotaSummary{}, err
	}
	summary := summarizeGroupQuota(accounts, time.Now())
	summary.GroupID = groupID
	return summary, nil
}

func summarizeGroupQuota(accounts []Account, now time.Time) GroupQuotaSummary {
	var remaining5h, remaining7d float64
	var known5h, known7d int
	var summary GroupQuotaSummary
	for _, account := range accounts {
		if !isGroupQuotaAccount(account, now) {
			continue
		}
		summary.AccountCount++
		if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
			summary.CooldownAccountCount++
		}
		if remaining, ok := accountQuotaRemaining(account, "5h", now); ok {
			remaining5h += remaining
			known5h++
		}
		if remaining, ok := accountQuotaRemaining(account, "7d", now); ok {
			remaining7d += remaining
			known7d++
		}
	}
	if known5h > 0 {
		remaining5h /= float64(known5h)
		summary.Remaining5hPercent = &remaining5h
	}
	if known7d > 0 {
		remaining7d /= float64(known7d)
		summary.Remaining7dPercent = &remaining7d
	}
	return summary
}

func isGroupQuotaAccount(account Account, now time.Time) bool {
	if account.Status != StatusActive || !account.Schedulable {
		return false
	}
	if account.AutoPauseOnExpired && account.ExpiresAt != nil && !now.Before(*account.ExpiresAt) {
		return false
	}
	if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
		return false
	}
	return true
}

func accountQuotaRemaining(account Account, window string, now time.Time) (float64, bool) {
	if progress := buildCodexUsageProgressFromExtra(account.Extra, window, now); progress != nil {
		return remainingQuotaPercent(progress.Utilization), true
	}

	var utilKey, resetKey string
	if window == "5h" {
		utilKey, resetKey = "session_window_utilization", ""
	} else if window == "7d" {
		utilKey, resetKey = "passive_usage_7d_utilization", "passive_usage_7d_reset"
	} else {
		return 0, false
	}
	utilization, ok := account.Extra[utilKey]
	if !ok {
		if window != "5h" || account.SessionWindowEnd == nil {
			return 0, false
		}
		switch account.SessionWindowStatus {
		case "rejected":
			utilization = 1.0
		case "allowed_warning":
			utilization = 0.8
		default:
			return 0, false
		}
	}
	used := parseExtraFloat64(utilization) * 100
	var resetAt *time.Time
	if window == "5h" && account.SessionWindowEnd != nil {
		resetAt = account.SessionWindowEnd
	} else if rawReset, exists := account.Extra[resetKey]; exists {
		reset := time.Unix(int64(parseExtraFloat64(rawReset)), 0)
		resetAt = &reset
	}
	if resetAt != nil && !now.Before(*resetAt) {
		used = 0
	}
	return remainingQuotaPercent(used), true
}

func remainingQuotaPercent(used float64) float64 {
	if used < 0 {
		used = 0
	}
	if used > 100 {
		used = 100
	}
	return 100 - used
}
