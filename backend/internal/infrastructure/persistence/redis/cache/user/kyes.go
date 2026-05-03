package user_cache

func UserInfoKey(userId string) string {
	return "user:info:" + userId
}
