package services

import (
	"context"

	apperrors "unwise-backend/errors"
	"unwise-backend/repository"
)

func RequireGroupMembership(ctx context.Context, groupRepo repository.GroupRepository, groupID, userID string) error {
	isMember, err := groupRepo.IsMember(ctx, groupID, userID)
	if err != nil {
		return apperrors.DatabaseError("checking membership", err)
	}
	if !isMember {
		return apperrors.NotGroupMember()
	}
	return nil
}