package room_valueobject

type Role int

var (
	RegularUser   Role = 1
	HomeOwner     Role = 2
	Administrator Role = 3
)
