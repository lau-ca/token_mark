package model

func DeleteUserForSession(identity AuthSessionIdentity) error {
	return DB.Where("id = ?", identity.UserID).Delete(&User{}).Error
}
