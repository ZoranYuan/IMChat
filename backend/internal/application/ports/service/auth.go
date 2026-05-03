package service

type AuthService interface {
	IssueToken(userId string) (string, string, error)
}
