package key

func UserInfoKey(userId string) string {
	return "user:info:" + userId
}
