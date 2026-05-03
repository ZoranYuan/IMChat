package friend_request_valueobject

type Status int

const (
	Pending Status = iota + 1
	Accepted
	Refused
)
