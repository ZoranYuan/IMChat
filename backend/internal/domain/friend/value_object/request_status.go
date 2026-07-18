package valueobject

type RequestStatus int

const (
	Pending RequestStatus = iota + 1
	Accepted
	Refused
)
