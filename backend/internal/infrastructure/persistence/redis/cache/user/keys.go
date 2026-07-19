package user

func UserProfileKey(userID string) string {
	return "im:user:profile:" + userID
}
