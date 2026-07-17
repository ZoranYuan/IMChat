package key

import "testing"

func TestBuildTrimsEmptyAndColonParts(t *testing.T) {
	got := Build("", ":room:", "123", ":members:")
	want := "im:room:123:members"
	if got != want {
		t.Fatalf("Build() = %q, want %q", got, want)
	}
}

func TestRedisKeyNaming(t *testing.T) {
	tests := map[string]string{
		AuthAccessToken("token-1"):    "im:auth:token:access:token-1",
		AuthAccessUser("u1"):          "im:auth:user:u1:access",
		AuthRefreshUser("u1"):         "im:auth:user:u1:refresh",
		AuthRefreshToken("refresh-1"): "im:auth:token:refresh:refresh-1",
		ConversationSeq("conv-1"):     "im:conversation:conv-1:seq",
		FriendRelation("u1", "u2"):    "im:friend:relation:u1:u2",
		FileMeta("file-1"):            "im:file:file-1:meta",
		UserInfo("u1"):                "im:user:u1:info",
		RoomInviteCode("123456789"):   "im:room:invite:123456789",
		RoomInvite("room-1"):          "im:room:room-1:invite",
		RoomMembers("room-1"):         "im:room:room-1:members",
		RoomMember("room-1", "u1"):    "im:room:room-1:member:u1",
	}

	for got, want := range tests {
		if got != want {
			t.Fatalf("key = %q, want %q", got, want)
		}
	}
}
