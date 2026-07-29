package service

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

const (
	QuotaForecastStatusPredicted     = "predicted"
	QuotaForecastStatusDepleted      = "depleted"
	QuotaForecastStatusSampling      = "sampling"
	QuotaForecastStatusNoRecentUsage = "no_recent_usage"
	QuotaForecastStatusUnavailable   = "unavailable"

	quotaForecastWindowCount = 7
	quotaForecastDaySeconds  = int64(24 * 60 * 60)
	quotaForecastCacheTTL    = 5 * time.Minute
	quotaForecastCacheLimit  = 4096
)

type QuotaForecastResult struct {
	UserID               int     `json:"user_id"`
	Status               string  `json:"status"`
	CalculatedAt         int64   `json:"calculated_at"`
	PredictedExhaustedAt int64   `json:"predicted_exhausted_at,omitempty"`
	WeightedDailyUsage   float64 `json:"weighted_daily_usage,omitempty"`
	RemainingSeconds     int64   `json:"remaining_seconds,omitempty"`
	SampleHours          int     `json:"sample_hours"`
}

type quotaForecastCacheEntry struct {
	result    QuotaForecastResult
	quota     int
	expiresAt time.Time
}

var quotaForecastCache = struct {
	sync.Mutex
	entries map[int]quotaForecastCacheEntry
}{entries: make(map[int]quotaForecastCacheEntry)}

func GetQuotaForecasts(ctx context.Context, userIDs []int, calculatedAt int64) ([]QuotaForecastResult, error) {
	startedAt := time.Now()
	users, err := model.GetQuotaForecastUsers(userIDs)
	if err != nil {
		return nil, err
	}

	results := make(map[int]QuotaForecastResult, len(users))
	usersByID := make(map[int]model.QuotaForecastUser, len(users))
	misses := make([]int, 0, len(users))
	now := time.Now()
	cacheHits := 0

	quotaForecastCache.Lock()
	for _, user := range users {
		usersByID[user.UserID] = user
		entry, ok := quotaForecastCache.entries[user.UserID]
		if ok && entry.quota == user.Quota && now.Before(entry.expiresAt) {
			results[user.UserID] = entry.result
			cacheHits++
			continue
		}
		misses = append(misses, user.UserID)
	}
	quotaForecastCache.Unlock()

	if len(misses) > 0 {
		usage, queryErr := model.GetQuotaForecastUsage(misses, calculatedAt-quotaForecastWindowCount*quotaForecastDaySeconds, calculatedAt)
		if queryErr != nil {
			return nil, queryErr
		}
		usageByUser := make(map[int][]model.QuotaForecastUsage, len(misses))
		for _, item := range usage {
			usageByUser[item.UserID] = append(usageByUser[item.UserID], item)
		}

		quotaForecastCache.Lock()
		for _, userID := range misses {
			user := usersByID[userID]
			result := calculateQuotaForecast(user, usageByUser[userID], calculatedAt)
			results[userID] = result
			quotaForecastCache.entries[userID] = quotaForecastCacheEntry{
				result:    result,
				quota:     user.Quota,
				expiresAt: now.Add(quotaForecastCacheTTL),
			}
		}
		trimQuotaForecastCacheLocked(now)
		quotaForecastCache.Unlock()
	}

	ordered := make([]QuotaForecastResult, 0, len(results))
	for _, userID := range userIDs {
		if result, ok := results[userID]; ok {
			ordered = append(ordered, result)
		}
	}
	logger.LogDebug(ctx, "quota forecast completed requested=%d resolved=%d cache_hits=%d duration_ms=%d", len(userIDs), len(ordered), cacheHits, time.Since(startedAt).Milliseconds())
	return ordered, nil
}

func calculateQuotaForecast(user model.QuotaForecastUser, usage []model.QuotaForecastUsage, calculatedAt int64) QuotaForecastResult {
	result := QuotaForecastResult{
		UserID:       user.UserID,
		Status:       QuotaForecastStatusUnavailable,
		CalculatedAt: calculatedAt,
	}
	if user.Quota <= 0 {
		result.Status = QuotaForecastStatusDepleted
		return result
	}

	historyStart := calculatedAt - quotaForecastWindowCount*quotaForecastDaySeconds
	if user.CreatedAt > historyStart {
		historyStart = user.CreatedAt
	}
	sampleSeconds := calculatedAt - historyStart
	if sampleSeconds <= 0 {
		return result
	}
	result.SampleHours = int(sampleSeconds / 3600)
	if sampleSeconds < quotaForecastDaySeconds {
		result.Status = QuotaForecastStatusSampling
		return result
	}

	windowUsage := [quotaForecastWindowCount]float64{}
	totalUsage := int64(0)
	for _, item := range usage {
		if item.CreatedAt < historyStart || item.CreatedAt >= calculatedAt || item.Quota <= 0 {
			continue
		}
		windowIndex := int((calculatedAt - 1 - item.CreatedAt) / quotaForecastDaySeconds)
		if windowIndex < 0 || windowIndex >= quotaForecastWindowCount {
			continue
		}
		windowUsage[windowIndex] += float64(item.Quota)
		totalUsage += item.Quota
	}
	if totalUsage == 0 {
		result.Status = QuotaForecastStatusNoRecentUsage
		return result
	}

	weightedUsage := 0.0
	effectiveWeight := 0.0
	for index := 0; index < quotaForecastWindowCount; index++ {
		windowEnd := calculatedAt - int64(index)*quotaForecastDaySeconds
		windowStart := windowEnd - quotaForecastDaySeconds
		availableStart := windowStart
		if historyStart > availableStart {
			availableStart = historyStart
		}
		availableSeconds := windowEnd - availableStart
		if availableSeconds <= 0 {
			continue
		}
		if availableSeconds > quotaForecastDaySeconds {
			availableSeconds = quotaForecastDaySeconds
		}
		weight := float64(quotaForecastWindowCount - index)
		weightedUsage += windowUsage[index] * weight
		effectiveWeight += weight * float64(availableSeconds) / float64(quotaForecastDaySeconds)
	}
	if effectiveWeight <= 0 {
		return result
	}

	dailyUsage := weightedUsage / effectiveWeight
	if dailyUsage <= 0 || math.IsNaN(dailyUsage) || math.IsInf(dailyUsage, 0) {
		return result
	}
	remainingSecondsFloat := float64(user.Quota) / dailyUsage * float64(quotaForecastDaySeconds)
	if remainingSecondsFloat <= 0 || math.IsNaN(remainingSecondsFloat) || math.IsInf(remainingSecondsFloat, 0) || remainingSecondsFloat > float64(math.MaxInt64-calculatedAt) {
		return result
	}

	remainingSeconds := int64(math.Ceil(remainingSecondsFloat))
	result.Status = QuotaForecastStatusPredicted
	result.WeightedDailyUsage = dailyUsage
	result.RemainingSeconds = remainingSeconds
	result.PredictedExhaustedAt = calculatedAt + remainingSeconds
	return result
}

func trimQuotaForecastCacheLocked(now time.Time) {
	for userID, entry := range quotaForecastCache.entries {
		if !now.Before(entry.expiresAt) {
			delete(quotaForecastCache.entries, userID)
		}
	}
	if len(quotaForecastCache.entries) <= quotaForecastCacheLimit {
		return
	}
	type cacheExpiry struct {
		userID    int
		expiresAt time.Time
	}
	expiries := make([]cacheExpiry, 0, len(quotaForecastCache.entries))
	for userID, entry := range quotaForecastCache.entries {
		expiries = append(expiries, cacheExpiry{userID: userID, expiresAt: entry.expiresAt})
	}
	sort.Slice(expiries, func(i int, j int) bool {
		return expiries[i].expiresAt.Before(expiries[j].expiresAt)
	})
	for index := 0; index < len(expiries)-quotaForecastCacheLimit; index++ {
		delete(quotaForecastCache.entries, expiries[index].userID)
	}
}
