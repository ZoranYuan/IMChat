package room_valueobject

type RoomUserStatus int

var (
	Activate RoomUserStatus = 1
	BeMuted  RoomUserStatus = 2
	BeKicked RoomUserStatus = 3
	Left     RoomUserStatus = 4
)
