package application_user_interface

type AuthService interface {
	IssueToken(userId string) (string, string, error)
}
