package user_valueobject

import "regexp"

type Phone string

func (p Phone) Validate() bool {
	pattern := `^1[3-9]\d{9}$`
	macthed, _ := regexp.MatchString(pattern, string(p))
	return macthed
}
