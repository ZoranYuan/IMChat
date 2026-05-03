package auth_service_interface

type AuthService interface {
	IssueToken(userId string) (string, string, error)
}
