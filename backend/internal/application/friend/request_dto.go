package friend

import (
	friendrequestentity "IM_backend/internal/domain/friend/entity"
)

type FriendRequestDTO struct {
	RequestID         string
	ApplicantUserID   string
	ApplicantNickName string
	Message           string
	Status            int
	ApplyTime         int64
}

func toFriendRequestDTO(fr *friendrequestentity.FriendRequest) FriendRequestDTO {
	return FriendRequestDTO{
		RequestID:       fr.RequestId,
		ApplicantUserID: fr.ApplicantUserId,
		Message:         fr.Message,
		Status:          int(fr.Status),
		ApplyTime:       fr.ApplyTime,
	}
}
