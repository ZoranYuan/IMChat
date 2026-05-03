package valueobject

type Role int

var (
	RegularUser   Role = 1
	Administrator Role = 2
	HomeOwner     Role = 3
)
