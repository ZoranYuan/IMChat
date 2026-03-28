package service_auth

type AuthService interface {
	IssueToken(userId string) (string, string, error)
}
