package key

func AccessTokenKey(token string) string {
	return "auth:access:token:" + token
}

func AccessUserKey(userId string) string {
	return "auth:access:user:" + userId
}

func RefreshUserKey(userId string) string {
	return "auth:refresh:user:" + userId
}

func RefreshTokenKey(token string) string {
	return "auth:refresh:token:" + token
}
