package model
func InvalidateUserCache(userId int) error { return invalidateUserCache(userId) }
func updateUserQuotaCache(userId int, quota int) error { return updateUserCacheField(userId, "Quota", quota) }
