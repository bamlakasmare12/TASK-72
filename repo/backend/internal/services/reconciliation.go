package services

import (
	"context"
	"fmt"
	"log"
	"math"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/repository"
	"wlpr-portal/pkg/crypto"
)

type ReconciliationService struct {
	procRepo  *repository.ProcurementRepository
	configSvc *ConfigService
}

func NewReconciliationService(procRepo *repository.ProcurementRepository, configSvc *ConfigService) *ReconciliationService {
	return &ReconciliationService{
		procRepo:  procRepo,
		configSvc: configSvc,
	}
}

// CompareStatement compares a vendor's statement total against ledger entries.
// If variance < threshold -> PENDING_APPROVAL (auto-suggest write-off).
// If variance >= threshold -> MANUAL_INVESTIGATION.
func (s *ReconciliationService) CompareStatement(ctx context.Context, req models.CompareStatementRequest) (*models.CompareStatementResponse, error) {
	if req.VendorID <= 0 {
		return nil, fmt.Errorf("vendor_id is required")
	}

	// Sum AR entries for the vendor in the period
	ledgerTotal, err := s.procRepo.SumLedgerByVendor(ctx, req.VendorID, "AR", req.PeriodStart, req.PeriodEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to sum ledger: %w", err)
	}

	variance := req.StatementTotal - ledgerTotal
	variancePct := 0.0
	if ledgerTotal != 0 {
		variancePct = math.Round((variance/ledgerTotal)*10000) / 100
	}

	// Get write-off threshold from config (default $5.00)
	threshold := 5.00
	if val, ok := s.configSvc.GetConfig("variance.auto_writeoff_threshold"); ok {
		fmt.Sscanf(val, "%f", &threshold)
	}

	absVariance := math.Abs(variance)
	autoWriteoff := absVariance < threshold && absVariance > 0
	suggestedState := "matched"
	if absVariance > 0 {
		if absVariance < threshold {
			suggestedState = "pending_approval"
		} else {
			suggestedState = "manual_investigation"
		}
	}

	return &models.CompareStatementResponse{
		VendorID:       req.VendorID,
		StatementTotal: req.StatementTotal,
		LedgerTotal:    ledgerTotal,
		Variance:       math.Round(variance*100) / 100,
		VariancePct:    variancePct,
		AutoWriteoff:   autoWriteoff,
		SuggestedState: suggestedState,
	}, nil
}

// MatchInvoice matches an invoice to an order, computing variance and auto-classifying.
func (s *ReconciliationService) MatchInvoice(ctx context.Context, invoiceID, orderID, userID int) error {
	order, err := s.procRepo.GetOrderByID(ctx, orderID)
	if err != nil || order == nil {
		return fmt.Errorf("order not found")
	}
	if order.Status != "fulfilled" && order.Status != "partially_fulfilled" && order.Status != "approved" {
		return fmt.Errorf("order must be in fulfilled/approved status to match; current: %s", order.Status)
	}

	invoice, err := s.procRepo.GetInvoiceByID(ctx, invoiceID)
	if err != nil || invoice == nil {
		return fmt.Errorf("invoice not found")
	}
	if invoice.Status != "pending" {
		return fmt.Errorf("invoice must be in pending status to match; current: %s", invoice.Status)
	}

	// The SQL in MatchInvoice computes variance and auto-classifies via the DB trigger
	return s.procRepo.MatchInvoice(ctx, invoiceID, orderID, userID)
}

// TransitionSettlement validates and executes settlement status transitions.
// Valid transitions:
//
//	open -> matched
//	matched -> variance_pending
//	variance_pending -> writeoff_suggested
//	writeoff_suggested -> writeoff_approved (requires Finance approval)
//	writeoff_approved -> settled
//	any open state -> disputed
//	matched -> settled (no variance)
func (s *ReconciliationService) TransitionSettlement(ctx context.Context, req models.SettlementTransitionRequest, userID int) error {
	settlement, err := s.procRepo.GetSettlementByID(ctx, req.SettlementID)
	if err != nil || settlement == nil {
		return fmt.Errorf("settlement not found")
	}

	currentStatus := settlement.Status
	var newStatus string

	switch req.Action {
	case "match":
		if currentStatus != "open" {
			return fmt.Errorf("invalid transition: cannot match from status %s", currentStatus)
		}
		newStatus = "matched"

	case "flag_variance":
		if currentStatus != "matched" {
			return fmt.Errorf("invalid transition: cannot flag variance from status %s", currentStatus)
		}
		newStatus = "variance_pending"

	case "suggest_writeoff":
		if currentStatus != "variance_pending" {
			return fmt.Errorf("invalid transition: cannot suggest write-off from status %s", currentStatus)
		}
		// Validate variance is below threshold for auto-suggest
		threshold := 5.00
		if val, ok := s.configSvc.GetConfig("variance.auto_writeoff_threshold"); ok {
			fmt.Sscanf(val, "%f", &threshold)
		}
		if math.Abs(settlement.VarianceAmount) >= threshold {
			return fmt.Errorf("variance $%.2f exceeds auto write-off threshold $%.2f; requires manual investigation", math.Abs(settlement.VarianceAmount), threshold)
		}
		newStatus = "writeoff_suggested"

	case "approve_writeoff":
		if currentStatus != "writeoff_suggested" {
			return fmt.Errorf("invalid transition: cannot approve write-off from status %s", currentStatus)
		}
		newStatus = "writeoff_approved"

	case "settle":
		if currentStatus != "matched" && currentStatus != "writeoff_approved" {
			return fmt.Errorf("invalid transition: cannot settle from status %s", currentStatus)
		}
		newStatus = "settled"

	case "dispute":
		if currentStatus == "settled" || currentStatus == "disputed" {
			return fmt.Errorf("invalid transition: cannot dispute from status %s", currentStatus)
		}
		newStatus = "disputed"

	default:
		return fmt.Errorf("unknown action: %s", req.Action)
	}

	return s.procRepo.UpdateSettlement(ctx, req.SettlementID, newStatus, &userID, req.Notes)
}

// TransitionDispute validates and executes dispute state transitions.
// The DB trigger trg_dispute_state_machine also enforces this, providing defense-in-depth.
//
// Valid transitions:
//
//	created -> evidence_uploaded
//	evidence_uploaded -> under_review
//	under_review -> arbitration
//	arbitration -> resolved_hidden | resolved_disclaimer | resolved_restored | rejected
func (s *ReconciliationService) TransitionDispute(ctx context.Context, req models.DisputeTransitionRequest, userID int) error {
	dispute, err := s.procRepo.GetDisputeByID(ctx, req.DisputeID)
	if err != nil || dispute == nil {
		return fmt.Errorf("dispute not found")
	}

	currentStatus := dispute.Status
	var newStatus string
	updates := make(map[string]interface{})

	switch req.Action {
	case "upload_evidence":
		if currentStatus != "created" {
			return fmt.Errorf("invalid transition: cannot upload evidence from status %s (must be 'created')", currentStatus)
		}
		if len(req.EvidenceURLs) == 0 {
			return fmt.Errorf("at least one evidence URL is required")
		}
		newStatus = "evidence_uploaded"
		updates["evidence_urls"] = req.EvidenceURLs

		// Encrypt evidence metadata at rest if provided
		if req.EvidenceMetadata != "" {
			encMeta, err := crypto.EncryptString(req.EvidenceMetadata)
			if err != nil {
				log.Printf("[reconciliation] evidence metadata encryption error: %v", err)
				return fmt.Errorf("failed to encrypt evidence metadata")
			}
			updates["evidence_metadata_enc"] = encMeta
		}

	case "start_review":
		if currentStatus != "evidence_uploaded" {
			return fmt.Errorf("invalid transition: cannot start review from status %s (must be 'evidence_uploaded')", currentStatus)
		}
		newStatus = "under_review"

	case "respond":
		if currentStatus != "evidence_uploaded" && currentStatus != "under_review" {
			return fmt.Errorf("merchant can only respond during evidence_uploaded or under_review phase")
		}
		if req.MerchantResponse == "" {
			return fmt.Errorf("merchant_response is required")
		}
		// Merchant response doesn't change state, just records the response
		updates["merchant_response"] = req.MerchantResponse
		newStatus = currentStatus // no state change for response

	case "escalate_arbitration":
		if currentStatus != "under_review" {
			return fmt.Errorf("invalid transition: cannot escalate to arbitration from status %s (must be 'under_review')", currentStatus)
		}
		newStatus = "arbitration"

	case "arbitrate":
		if currentStatus != "arbitration" {
			return fmt.Errorf("invalid transition: cannot arbitrate from status %s (must be 'arbitration')", currentStatus)
		}
		outcome := req.ArbitrationOutcome
		if outcome != "hide" && outcome != "disclaimer" && outcome != "restore" {
			return fmt.Errorf("arbitration_outcome must be 'hide', 'disclaimer', or 'restore'")
		}

		switch outcome {
		case "hide":
			newStatus = "resolved_hidden"
		case "disclaimer":
			newStatus = "resolved_disclaimer"
		case "restore":
			newStatus = "resolved_restored"
		}

		updates["arbitration_notes"] = req.ArbitrationNotes
		updates["arbitration_outcome"] = outcome
		updates["arbitrated_by"] = userID

	case "reject":
		if currentStatus != "arbitration" {
			return fmt.Errorf("invalid transition: cannot reject from status %s (must be 'arbitration')", currentStatus)
		}
		newStatus = "rejected"
		updates["arbitration_notes"] = req.ArbitrationNotes
		updates["arbitrated_by"] = userID

	default:
		return fmt.Errorf("unknown dispute action: %s", req.Action)
	}

	// If merchant response (no state change), handle separately
	if req.Action == "respond" {
		return s.procRepo.TransitionDispute(ctx, req.DisputeID, currentStatus, updates)
	}

	err = s.procRepo.TransitionDispute(ctx, req.DisputeID, newStatus, updates)
	if err != nil {
		return fmt.Errorf("transition failed: %w", err)
	}

	// Apply review status change based on arbitration outcome
	if req.Action == "arbitrate" || req.Action == "reject" {
		return s.applyArbitrationOutcome(ctx, dispute.ReviewID, newStatus, req.ArbitrationNotes)
	}

	return nil
}

// applyArbitrationOutcome updates the vendor review based on dispute resolution.
func (s *ReconciliationService) applyArbitrationOutcome(ctx context.Context, reviewID int, disputeStatus, notes string) error {
	switch disputeStatus {
	case "resolved_hidden":
		return s.procRepo.UpdateReviewStatus(ctx, reviewID, "hidden", "")
	case "resolved_disclaimer":
		disclaimer := "This review is subject to a resolved dispute."
		if notes != "" {
			disclaimer = notes
		}
		return s.procRepo.UpdateReviewStatus(ctx, reviewID, "disclaimer", disclaimer)
	case "resolved_restored":
		return s.procRepo.UpdateReviewStatus(ctx, reviewID, "visible", "")
	case "rejected":
		// Dispute rejected = review stays visible
		return s.procRepo.UpdateReviewStatus(ctx, reviewID, "visible", "")
	}
	return nil
}

// GetLedgerForExport returns all ledger entries for a vendor, ready for streaming.
func (s *ReconciliationService) GetLedgerForExport(ctx context.Context, vendorID int) ([]models.LedgerEntry, error) {
	return s.procRepo.GetLedgerEntries(ctx, vendorID, "")
}

// GetSettlementsForExport returns all settlements, ready for streaming.
func (s *ReconciliationService) GetSettlementsForExport(ctx context.Context) ([]models.Settlement, error) {
	return s.procRepo.GetSettlements(ctx, "")
}
