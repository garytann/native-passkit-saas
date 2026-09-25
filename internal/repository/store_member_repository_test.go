package repository

import (
	"errors"
	"testing"

	"native-ios-passkit-wip/internal/domain"
)

func TestNextStampStateUsesConfiguredGoal(t *testing.T) {
	tests := []struct {
		name         string
		previous     int
		delta        int
		goal         int
		status       string
		wantProgress int
		wantStatus   string
	}{
		{name: "below goal remains active", previous: 9, delta: 1, goal: 12, status: "active", wantProgress: 10, wantStatus: "active"},
		{name: "goal becomes reward ready", previous: 11, delta: 1, goal: 12, status: "active", wantProgress: 12, wantStatus: "reward_ready"},
		{name: "progress clamps at goal", previous: 12, delta: 5, goal: 12, status: "reward_ready", wantProgress: 12, wantStatus: "reward_ready"},
		{name: "progress clamps at zero", previous: 1, delta: -5, goal: 12, status: "active", wantProgress: 0, wantStatus: "active"},
		{name: "moving below goal restores active", previous: 12, delta: -1, goal: 12, status: "reward_ready", wantProgress: 11, wantStatus: "active"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			progress, status := nextStampState(test.previous, test.delta, test.goal, test.status)
			if progress != test.wantProgress || status != test.wantStatus {
				t.Fatalf("next stamp state = (%d, %q), want (%d, %q)", progress, status, test.wantProgress, test.wantStatus)
			}
		})
	}
}

func TestApplyRewardStateIssuesCompletedCard(t *testing.T) {
	member := storeMemberModel{StampProgress: 12, AvailableRewards: 2, Status: "reward_ready"}
	if err := applyRewardState(&member, "issue", 12); err != nil {
		t.Fatalf("issue reward: %v", err)
	}
	if member.StampProgress != 0 || member.AvailableRewards != 3 || member.Status != "active" {
		t.Fatalf("unexpected issued state: %+v", member)
	}
}

func TestApplyRewardStateRedeemsWithoutChangingStamps(t *testing.T) {
	member := storeMemberModel{StampProgress: 4, AvailableRewards: 2, Status: "active"}
	if err := applyRewardState(&member, "redeem", 12); err != nil {
		t.Fatalf("redeem reward: %v", err)
	}
	if member.StampProgress != 4 || member.AvailableRewards != 1 || member.Status != "active" {
		t.Fatalf("unexpected redeemed state: %+v", member)
	}
}

func TestApplyRewardStateRejectsInvalidBalances(t *testing.T) {
	if err := applyRewardState(&storeMemberModel{StampProgress: 11, Status: "active"}, "issue", 12); !errors.Is(err, domain.ErrRewardNotReady) {
		t.Fatalf("expected reward not ready, got %v", err)
	}
	if err := applyRewardState(&storeMemberModel{AvailableRewards: 0, Status: "active"}, "redeem", 12); !errors.Is(err, domain.ErrNoAvailableRewards) {
		t.Fatalf("expected no rewards, got %v", err)
	}
	if err := applyRewardState(&storeMemberModel{StampProgress: 12, Status: "inactive"}, "issue", 12); !errors.Is(err, domain.ErrMemberInactive) {
		t.Fatalf("expected inactive member, got %v", err)
	}
}
