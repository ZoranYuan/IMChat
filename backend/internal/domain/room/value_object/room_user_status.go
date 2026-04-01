package room_valueobject

type RoomUserStatus int

var (
	Activate RoomUserStatus = 1
	BeMuted  RoomUserStatus = 2
)
