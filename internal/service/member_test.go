package service

import (
	"context"
	"errors"
	"testing"

	"native-ios-passkit-wip/internal/domain"
)

type fakeMemberRepository struct {
	updateErr error
}

func (fakeMemberRepository) Create(context.Context, domain.CreateStoreMemberInput) (domain.StoreMember, error) {
	return domain.StoreMember{}, nil
}

func (fakeMemberRepository) GetByID(context.Context, string) (domain.StoreMember, error) {
	return domain.StoreMember{}, nil
}

func (fakeMemberRepository) ListByMerchant(context.Context, string) ([]domain.StoreMember, error) {
	return nil, nil
}

func (r fakeMemberRepository) UpdateStampProgressForMerchant(context.Context, string, string, int) (domain.StampUpdate, error) {
	if r.updateErr != nil {
		return domain.StampUpdate{}, r.updateErr
	}
	return domain.StampUpdate{}, nil
}

func TestUpdateMemberStampsMapsMerchantMismatchToForbidden(t *testing.T) {
	service := NewMemberService(fakeMemberRepository{updateErr: domain.ErrStoreMemberForbidden}, nil)

	_, err := service.UpdateMemberStamps(context.Background(), UpdateMemberStampsInput{
		MemberID:   "MBR-ABC12345",
		Merchant:   "Daily Brew",
		StampDelta: 1,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}
