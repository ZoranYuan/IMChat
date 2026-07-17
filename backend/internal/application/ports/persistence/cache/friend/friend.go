package friend

import (
	friendvo "IM_backend/internal/domain/friend/value_object"
	"context"
)

type RelationState struct {
	Status friendvo.Status
}

type FriendCache interface {
	GetRelation(ctx context.Context, userID, friendID string) (*RelationState, bool, error)
	SetRelation(ctx context.Context, userID, friendID string, state *RelationState) error
	SetRelationNotFound(ctx context.Context, userID, friendID string) error
	DeleteRelation(ctx context.Context, userID, friendID string) error
}
