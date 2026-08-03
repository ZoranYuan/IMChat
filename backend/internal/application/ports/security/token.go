package security

type TokenIssuer interface {
	IssueToken(userId string) (accessToken string, refreshToken string, sessionID string, err error)
}
