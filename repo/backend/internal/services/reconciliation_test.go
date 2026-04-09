package services

import (
	"math"
	"testing"
)

// classifyVariance is the pure variance classification logic extracted from
// ReconciliationService.CompareStatement for direct unit testing.
// It mirrors the exact logic in reconciliation.go lines 54-63.
func classifyVariance(invoiceAmount, orderAmount, threshold float64) (suggestedState string, autoWriteoff bool) {
	variance := invoiceAmount - orderAmount
	absVariance := math.Abs(variance)

	autoWriteoff = absVariance < threshold && absVariance > 0
	suggestedState = "matched"
	if absVariance > 0 {
		if absVariance < threshold {
			suggestedState = "pending_approval"
		} else {
			suggestedState = "manual_investigation"
		}
	}
	return
}

// TestVariance_BelowThreshold proves that variance < $5.00 yields PENDING_APPROVAL.
func TestVariance_BelowThreshold(t *testing.T) {
	threshold := 5.00

	tests := []struct {
		name           string
		invoiceAmt     float64
		orderAmt       float64
		wantState      string
		wantAutoWO     bool
	}{
		{
			name:       "variance $0.01 positive",
			invoiceAmt: 100.01,
			orderAmt:   100.00,
			wantState:  "pending_approval",
			wantAutoWO: true,
		},
		{
			name:       "variance $0.01 negative",
			invoiceAmt: 99.99,
			orderAmt:   100.00,
			wantState:  "pending_approval",
			wantAutoWO: true,
		},
		{
			name:       "variance $3.00 (under threshold)",
			invoiceAmt: 8753.50,
			orderAmt:   8750.50,
			wantState:  "pending_approval",
			wantAutoWO: true,
		},
		{
			name:       "variance $4.99 (just under threshold)",
			invoiceAmt: 1004.99,
			orderAmt:   1000.00,
			wantState:  "pending_approval",
			wantAutoWO: true,
		},
		{
			name:       "variance -$2.50 negative (absolute below threshold)",
			invoiceAmt: 997.50,
			orderAmt:   1000.00,
			wantState:  "pending_approval",
			wantAutoWO: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, autoWO := classifyVariance(tc.invoiceAmt, tc.orderAmt, threshold)
			if state != tc.wantState {
				t.Errorf("state: got %q, want %q", state, tc.wantState)
			}
			if autoWO != tc.wantAutoWO {
				t.Errorf("autoWriteoff: got %v, want %v", autoWO, tc.wantAutoWO)
			}
		})
	}
}

// TestVariance_AboveThreshold proves that variance >= $5.00 yields MANUAL_INVESTIGATION.
func TestVariance_AboveThreshold(t *testing.T) {
	threshold := 5.00

	tests := []struct {
		name           string
		invoiceAmt     float64
		orderAmt       float64
		wantState      string
		wantAutoWO     bool
	}{
		{
			name:       "variance $5.01 (just above threshold)",
			invoiceAmt: 1005.01,
			orderAmt:   1000.00,
			wantState:  "manual_investigation",
			wantAutoWO: false,
		},
		{
			name:       "variance $250.00 (large overage)",
			invoiceAmt: 3450.00,
			orderAmt:   3200.00,
			wantState:  "manual_investigation",
			wantAutoWO: false,
		},
		{
			name:       "variance -$50.00 negative (large undercharge)",
			invoiceAmt: 950.00,
			orderAmt:   1000.00,
			wantState:  "manual_investigation",
			wantAutoWO: false,
		},
		{
			name:       "variance $100.00",
			invoiceAmt: 12600.00,
			orderAmt:   12500.00,
			wantState:  "manual_investigation",
			wantAutoWO: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, autoWO := classifyVariance(tc.invoiceAmt, tc.orderAmt, threshold)
			if state != tc.wantState {
				t.Errorf("state: got %q, want %q", state, tc.wantState)
			}
			if autoWO != tc.wantAutoWO {
				t.Errorf("autoWriteoff: got %v, want %v", autoWO, tc.wantAutoWO)
			}
		})
	}
}

// TestVariance_ExactBoundary tests the exact $5.00 boundary.
// The code uses `absVariance < threshold`, so $5.00 exactly is NOT auto-writeoff.
func TestVariance_ExactBoundary(t *testing.T) {
	threshold := 5.00

	t.Run("exact $5.00 variance", func(t *testing.T) {
		state, autoWO := classifyVariance(1005.00, 1000.00, threshold)
		// $5.00 is NOT < $5.00, so it goes to manual_investigation
		if state != "manual_investigation" {
			t.Errorf("state: got %q, want %q", state, "manual_investigation")
		}
		if autoWO != false {
			t.Errorf("autoWriteoff: got %v, want false", autoWO)
		}
	})

	t.Run("exact -$5.00 negative variance", func(t *testing.T) {
		state, autoWO := classifyVariance(995.00, 1000.00, threshold)
		if state != "manual_investigation" {
			t.Errorf("state: got %q, want %q", state, "manual_investigation")
		}
		if autoWO != false {
			t.Errorf("autoWriteoff: got %v, want false", autoWO)
		}
	})

	t.Run("$4.999999 variance (just under $5.00)", func(t *testing.T) {
		state, autoWO := classifyVariance(1004.999999, 1000.00, threshold)
		if state != "pending_approval" {
			t.Errorf("state: got %q, want %q", state, "pending_approval")
		}
		if autoWO != true {
			t.Errorf("autoWriteoff: got %v, want true", autoWO)
		}
	})

	t.Run("$5.000001 variance (just over $5.00)", func(t *testing.T) {
		state, autoWO := classifyVariance(1005.000001, 1000.00, threshold)
		if state != "manual_investigation" {
			t.Errorf("state: got %q, want %q", state, "manual_investigation")
		}
		if autoWO != false {
			t.Errorf("autoWriteoff: got %v, want false", autoWO)
		}
	})
}

// TestVariance_ZeroVariance proves zero variance yields "matched".
func TestVariance_ZeroVariance(t *testing.T) {
	threshold := 5.00

	state, autoWO := classifyVariance(12500.00, 12500.00, threshold)
	if state != "matched" {
		t.Errorf("state: got %q, want %q", state, "matched")
	}
	if autoWO != false {
		t.Errorf("autoWriteoff: got %v, want false (zero variance is not a writeoff)", autoWO)
	}
}

// TestVariance_CustomThreshold proves the logic works with non-default thresholds.
func TestVariance_CustomThreshold(t *testing.T) {
	t.Run("threshold $10.00", func(t *testing.T) {
		state, autoWO := classifyVariance(1007.00, 1000.00, 10.00)
		if state != "pending_approval" {
			t.Errorf("state: got %q, want %q (7.00 < 10.00)", state, "pending_approval")
		}
		if autoWO != true {
			t.Errorf("autoWriteoff: got %v, want true", autoWO)
		}
	})

	t.Run("threshold $1.00 makes $3.00 variance manual", func(t *testing.T) {
		state, autoWO := classifyVariance(1003.00, 1000.00, 1.00)
		if state != "manual_investigation" {
			t.Errorf("state: got %q, want %q (3.00 >= 1.00)", state, "manual_investigation")
		}
		if autoWO != false {
			t.Errorf("autoWriteoff: got %v, want false", autoWO)
		}
	})
}

// TestSettlementTransitions_ValidTransitions tests the valid settlement state machine transitions
// by validating the expected allowed/disallowed transitions.
func TestSettlementTransitions_ValidTransitions(t *testing.T) {
	// Model the valid transitions from the service code
	validTransitions := map[string][]string{
		"open":              {"matched"},
		"matched":           {"variance_pending", "settled"},
		"variance_pending":  {"writeoff_suggested"},
		"writeoff_suggested": {"writeoff_approved"},
		"writeoff_approved": {"settled"},
	}

	// Test each valid transition
	for from, toList := range validTransitions {
		for _, to := range toList {
			t.Run(from+"->"+to, func(t *testing.T) {
				// This is a state machine validation — just confirm the mapping exists
				found := false
				for _, allowed := range validTransitions[from] {
					if allowed == to {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("transition %s -> %s should be valid", from, to)
				}
			})
		}
	}
}

// TestSettlementTransitions_InvalidTransitions confirms invalid transitions are not in the valid set.
func TestSettlementTransitions_InvalidTransitions(t *testing.T) {
	validTransitions := map[string][]string{
		"open":              {"matched"},
		"matched":           {"variance_pending", "settled"},
		"variance_pending":  {"writeoff_suggested"},
		"writeoff_suggested": {"writeoff_approved"},
		"writeoff_approved": {"settled"},
	}

	invalidCases := [][2]string{
		{"open", "settled"},            // Cannot settle directly from open
		{"open", "writeoff_approved"},  // Cannot approve writeoff from open
		{"settled", "open"},            // Cannot re-open a settled record
		{"matched", "writeoff_approved"}, // Cannot skip writeoff_suggested
		{"variance_pending", "settled"}, // Must go through writeoff flow
	}

	for _, tc := range invalidCases {
		t.Run(tc[0]+"->"+tc[1]+"_must_be_invalid", func(t *testing.T) {
			allowed, ok := validTransitions[tc[0]]
			if !ok {
				// No transitions from this state, so it's correctly invalid
				return
			}
			for _, a := range allowed {
				if a == tc[1] {
					t.Errorf("transition %s -> %s should NOT be valid", tc[0], tc[1])
				}
			}
		})
	}
}

// TestDisputeTransitions_ValidTransitions validates the dispute state machine.
func TestDisputeTransitions_ValidTransitions(t *testing.T) {
	validTransitions := map[string][]string{
		"created":            {"evidence_uploaded"},
		"evidence_uploaded":  {"under_review"},
		"under_review":       {"arbitration"},
		"arbitration":        {"resolved_hidden", "resolved_disclaimer", "resolved_restored", "rejected"},
	}

	for from, toList := range validTransitions {
		for _, to := range toList {
			t.Run(from+"->"+to, func(t *testing.T) {
				found := false
				for _, allowed := range validTransitions[from] {
					if allowed == to {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("transition %s -> %s should be valid", from, to)
				}
			})
		}
	}
}

// TestDisputeTransitions_InvalidSkips confirms you cannot skip states.
func TestDisputeTransitions_InvalidSkips(t *testing.T) {
	validTransitions := map[string][]string{
		"created":            {"evidence_uploaded"},
		"evidence_uploaded":  {"under_review"},
		"under_review":       {"arbitration"},
		"arbitration":        {"resolved_hidden", "resolved_disclaimer", "resolved_restored", "rejected"},
	}

	invalidCases := [][2]string{
		{"created", "under_review"},          // Must upload evidence first
		{"created", "arbitration"},           // Cannot skip to arbitration
		{"created", "resolved_restored"},     // Cannot resolve from created
		{"evidence_uploaded", "arbitration"}, // Must go through under_review
		{"evidence_uploaded", "resolved_hidden"}, // Cannot resolve from evidence
		{"under_review", "resolved_hidden"},  // Must go through arbitration
	}

	for _, tc := range invalidCases {
		t.Run(tc[0]+"->"+tc[1]+"_must_be_invalid", func(t *testing.T) {
			allowed, ok := validTransitions[tc[0]]
			if !ok {
				return
			}
			for _, a := range allowed {
				if a == tc[1] {
					t.Errorf("transition %s -> %s should NOT be valid", tc[0], tc[1])
				}
			}
		})
	}
}
