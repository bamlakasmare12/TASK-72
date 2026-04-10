package services_test

import (
	"testing"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/services"
)

func TestIsFlagEnabled_AllStrategy(t *testing.T) {
	svc := services.NewConfigServiceWithFlags(make(map[string]models.Config), map[string]models.FeatureFlag{
		"test_flag": {Key: "test_flag", Enabled: true, RolloutStrategy: "all"},
	})

	if !svc.IsFlagEnabled("test_flag", 1, []int{1}) {
		t.Error("'all' strategy with enabled=true should return true")
	}
	if !svc.IsFlagEnabled("test_flag", 0, nil) {
		t.Error("'all' strategy should return true even without user context")
	}
}

func TestIsFlagEnabled_DisabledStrategy(t *testing.T) {
	svc := services.NewConfigServiceWithFlags(make(map[string]models.Config), map[string]models.FeatureFlag{
		"test_flag": {Key: "test_flag", Enabled: true, RolloutStrategy: "disabled"},
	})

	if svc.IsFlagEnabled("test_flag", 1, []int{1}) {
		t.Error("'disabled' strategy should always return false")
	}
}

func TestIsFlagEnabled_DisabledFlag(t *testing.T) {
	svc := services.NewConfigServiceWithFlags(make(map[string]models.Config), map[string]models.FeatureFlag{
		"test_flag": {Key: "test_flag", Enabled: false, RolloutStrategy: "all"},
	})

	if svc.IsFlagEnabled("test_flag", 1, []int{1}) {
		t.Error("disabled flag should return false regardless of strategy")
	}
}

func TestIsFlagEnabled_NonExistentFlag(t *testing.T) {
	svc := services.NewConfigServiceWithFlags(make(map[string]models.Config), make(map[string]models.FeatureFlag))

	if svc.IsFlagEnabled("nonexistent", 1, []int{1}) {
		t.Error("non-existent flag should return false")
	}
}

func TestIsFlagEnabled_RoleBased_WithMatchingRole(t *testing.T) {
	svc := services.NewConfigServiceWithFlags(make(map[string]models.Config), map[string]models.FeatureFlag{
		"test_flag": {Key: "test_flag", Enabled: true, RolloutStrategy: "role_based", AllowedRoles: []int{1, 3}},
	})

	if !svc.IsFlagEnabled("test_flag", 1, []int{3}) {
		t.Error("role_based with matching role should return true")
	}
}

func TestIsFlagEnabled_RoleBased_NoMatchingRole(t *testing.T) {
	svc := services.NewConfigServiceWithFlags(make(map[string]models.Config), map[string]models.FeatureFlag{
		"test_flag": {Key: "test_flag", Enabled: true, RolloutStrategy: "role_based", AllowedRoles: []int{1, 3}},
	})

	if svc.IsFlagEnabled("test_flag", 1, []int{2, 5}) {
		t.Error("role_based without matching role should return false")
	}
}

func TestIsFlagEnabled_RoleBased_NilRoles(t *testing.T) {
	svc := services.NewConfigServiceWithFlags(make(map[string]models.Config), map[string]models.FeatureFlag{
		"test_flag": {Key: "test_flag", Enabled: true, RolloutStrategy: "role_based", AllowedRoles: []int{1}},
	})

	if svc.IsFlagEnabled("test_flag", 1, nil) {
		t.Error("role_based with nil roles should return false")
	}
}

func TestIsFlagEnabled_Percentage_DeterministicBucketing(t *testing.T) {
	svc := services.NewConfigServiceWithFlags(make(map[string]models.Config), map[string]models.FeatureFlag{
		"test_flag": {Key: "test_flag", Enabled: true, RolloutStrategy: "percentage", RolloutPercentage: 50},
	})

	// Test deterministic bucketing: same user+flag always gets the same result
	result1 := svc.IsFlagEnabled("test_flag", 42, nil)
	result2 := svc.IsFlagEnabled("test_flag", 42, nil)
	if result1 != result2 {
		t.Error("percentage rollout must be deterministic for the same user+flag")
	}

	// Different users should get consistent (potentially different) results
	resultA := svc.IsFlagEnabled("test_flag", 100, nil)
	resultA2 := svc.IsFlagEnabled("test_flag", 100, nil)
	if resultA != resultA2 {
		t.Error("same user should always get the same result")
	}
}

func TestIsFlagEnabled_Percentage_ZeroDisabled(t *testing.T) {
	svc := services.NewConfigServiceWithFlags(make(map[string]models.Config), map[string]models.FeatureFlag{
		"test_flag": {Key: "test_flag", Enabled: true, RolloutStrategy: "percentage", RolloutPercentage: 0},
	})

	if svc.IsFlagEnabled("test_flag", 42, nil) {
		t.Error("0% rollout should always return false")
	}
}

func TestIsFlagEnabled_Percentage_HundredEnabled(t *testing.T) {
	svc := services.NewConfigServiceWithFlags(make(map[string]models.Config), map[string]models.FeatureFlag{
		"test_flag": {Key: "test_flag", Enabled: true, RolloutStrategy: "percentage", RolloutPercentage: 100},
	})

	if !svc.IsFlagEnabled("test_flag", 42, nil) {
		t.Error("100% rollout should always return true")
	}
}

func TestIsFlagEnabled_Percentage_NoUserID(t *testing.T) {
	svc := services.NewConfigServiceWithFlags(make(map[string]models.Config), map[string]models.FeatureFlag{
		"test_flag": {Key: "test_flag", Enabled: true, RolloutStrategy: "percentage", RolloutPercentage: 50},
	})

	if svc.IsFlagEnabled("test_flag", 0, nil) {
		t.Error("percentage rollout with userID=0 should return false")
	}
}

func TestIsFlagEnabled_Percentage_DistributionIsReasonable(t *testing.T) {
	svc := services.NewConfigServiceWithFlags(make(map[string]models.Config), map[string]models.FeatureFlag{
		"test_flag": {Key: "test_flag", Enabled: true, RolloutStrategy: "percentage", RolloutPercentage: 50},
	})

	enabledCount := 0
	totalUsers := 1000
	for i := 1; i <= totalUsers; i++ {
		if svc.IsFlagEnabled("test_flag", i, nil) {
			enabledCount++
		}
	}

	// With 50% rollout over 1000 users, we expect roughly 500 enabled.
	// Allow 35%-65% range to account for hash distribution variance.
	ratio := float64(enabledCount) / float64(totalUsers)
	if ratio < 0.35 || ratio > 0.65 {
		t.Errorf("50%% rollout over %d users: expected ~50%% enabled, got %.1f%% (%d/%d)",
			totalUsers, ratio*100, enabledCount, totalUsers)
	}
}
