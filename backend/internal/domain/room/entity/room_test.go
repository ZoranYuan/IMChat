package entity

import "testing"

func TestNewRoomUsesConfiguredMemberLimit(t *testing.T) {
	room, err := NewRoom("room-1", "user-1", "", "test", "")
	if err != nil {
		t.Fatalf("NewRoom() error = %v", err)
	}
	if room.MaxMembers != MaxMembers {
		t.Fatalf("MaxMembers = %d, want %d", room.MaxMembers, MaxMembers)
	}
}
