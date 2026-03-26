package friend_request_valueobject

type Status int

const (
	Pedding Status = iota + 1
	Accepted
	Refused
)
