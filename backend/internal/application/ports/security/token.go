package security

type TokenIssuer interface {
	IssueToken(userId string) (string, string, error)
}
