package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrTokenQuotaInsufficient = errors.New("token quota is not enough")
var ErrTokenQuotaPeriodChanged = errors.New("token daily quota period changed")

func NextTokenDailyQuotaReset(now time.Time) int64 {
	localNow := now.In(time.Local)
	return time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.Local).
		AddDate(0, 0, 1).
		Unix()
}

func refreshDailyQuotaTokenCache(token Token) {
	if !common.RedisEnabled {
		return
	}
	if err := cacheSetToken(token); err != nil {
		common.SysLog("failed to refresh daily quota token cache: " + err.Error())
	}
}

func resetDailyQuotaToken(token *Token, now int64) {
	token.RemainQuota = token.DailyQuota
	token.UsedQuota = 0
	if token.Status == common.TokenStatusExhausted {
		token.Status = common.TokenStatusEnabled
	}
	token.DailyQuotaNextResetTime = NextTokenDailyQuotaReset(time.Unix(now, 0))
}

func EnsureTokenDailyQuota(token *Token) error {
	if token == nil || !token.DailyQuotaEnabled {
		return nil
	}
	now := GetDBTimestamp()
	if token.DailyQuotaNextResetTime > now {
		return nil
	}

	var refreshed Token
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", token.Id).First(&refreshed).Error; err != nil {
			return err
		}
		if !refreshed.DailyQuotaEnabled || refreshed.DailyQuotaNextResetTime > now {
			return nil
		}
		resetDailyQuotaToken(&refreshed, now)
		return tx.Model(&refreshed).Select(
			"remain_quota",
			"used_quota",
			"status",
			"daily_quota_next_reset_time",
		).Updates(&refreshed).Error
	})
	if err != nil {
		return err
	}
	*token = refreshed
	refreshDailyQuotaTokenCache(refreshed)
	return nil
}

func ReserveTokenQuota(token *Token, quota int, expectedPeriod int64) (int64, error) {
	if quota < 0 {
		return 0, errors.New("quota 不能为负数！")
	}
	if token == nil {
		return 0, errors.New("token is nil")
	}
	if !token.DailyQuotaEnabled {
		if !token.UnlimitedQuota && token.RemainQuota < quota {
			return 0, ErrTokenQuotaInsufficient
		}
		return 0, DecreaseTokenQuota(token.Id, token.Key, quota)
	}
	if err := EnsureTokenDailyQuota(token); err != nil {
		return 0, err
	}
	if expectedPeriod > 0 && token.DailyQuotaNextResetTime != expectedPeriod {
		return 0, ErrTokenQuotaPeriodChanged
	}
	if quota == 0 {
		return token.DailyQuotaNextResetTime, nil
	}

	for attempt := 0; attempt < 2; attempt++ {
		period := token.DailyQuotaNextResetTime
		result := DB.Model(&Token{}).
			Where("id = ? AND daily_quota_enabled = ? AND daily_quota_next_reset_time = ? AND remain_quota >= ?", token.Id, true, period, quota).
			Updates(map[string]interface{}{
				"remain_quota":  gorm.Expr("remain_quota - ?", quota),
				"used_quota":    gorm.Expr("used_quota + ?", quota),
				"accessed_time": common.GetTimestamp(),
			})
		if result.Error != nil {
			return 0, result.Error
		}

		var refreshed Token
		if err := DB.First(&refreshed, token.Id).Error; err != nil {
			return 0, err
		}
		*token = refreshed
		refreshDailyQuotaTokenCache(refreshed)
		if result.RowsAffected == 1 {
			return period, nil
		}
		if err := EnsureTokenDailyQuota(token); err != nil {
			return 0, err
		}
		if expectedPeriod > 0 && token.DailyQuotaNextResetTime != expectedPeriod {
			return 0, ErrTokenQuotaPeriodChanged
		}
		if token.DailyQuotaNextResetTime == period || token.RemainQuota < quota {
			break
		}
	}
	return 0, ErrTokenQuotaInsufficient
}

func AdjustTokenQuotaForPeriod(tokenId int, key string, delta int, period int64) error {
	if delta == 0 {
		return nil
	}
	if period == 0 {
		if delta > 0 {
			return DecreaseTokenQuota(tokenId, key, delta)
		}
		return IncreaseTokenQuota(tokenId, key, -delta)
	}

	var refreshed Token
	now := GetDBTimestamp()
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", tokenId).First(&refreshed).Error; err != nil {
			return err
		}
		if refreshed.DailyQuotaEnabled && refreshed.DailyQuotaNextResetTime <= now {
			resetDailyQuotaToken(&refreshed, now)
			if err := tx.Model(&refreshed).Select(
				"remain_quota",
				"used_quota",
				"status",
				"daily_quota_next_reset_time",
			).Updates(&refreshed).Error; err != nil {
				return err
			}
		}
		if !refreshed.DailyQuotaEnabled || refreshed.DailyQuotaNextResetTime != period {
			return nil
		}
		if delta > 0 {
			if refreshed.RemainQuota < delta {
				return ErrTokenQuotaInsufficient
			}
			refreshed.RemainQuota -= delta
			refreshed.UsedQuota += delta
		} else {
			refund := -delta
			if refund > refreshed.UsedQuota {
				refund = refreshed.UsedQuota
			}
			availableSpace := refreshed.DailyQuota - refreshed.RemainQuota
			if refund > availableSpace {
				refund = availableSpace
			}
			refreshed.RemainQuota += refund
			refreshed.UsedQuota -= refund
		}
		refreshed.AccessedTime = common.GetTimestamp()
		return tx.Model(&refreshed).Select(
			"remain_quota",
			"used_quota",
			"accessed_time",
		).Updates(&refreshed).Error
	})
	if err != nil {
		return err
	}
	refreshDailyQuotaTokenCache(refreshed)
	return nil
}
