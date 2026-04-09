package models

import "time"

// ---- Vendors ----

type Vendor struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Code         string    `json:"code"`
	ContactEmail *string   `json:"contact_email,omitempty"`
	ContactPhone *string   `json:"contact_phone,omitempty"`
	Address      *string   `json:"address,omitempty"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ---- Orders ----

type ProcurementOrder struct {
	ID          int              `json:"id"`
	OrderNumber string           `json:"order_number"`
	VendorID    int              `json:"vendor_id"`
	VendorName  string           `json:"vendor_name,omitempty"`
	Status      string           `json:"status"`
	Department  *string          `json:"department,omitempty"`
	CostCenter  *string          `json:"cost_center,omitempty"`
	TotalAmount float64          `json:"total_amount"`
	Currency    string           `json:"currency"`
	Description *string          `json:"description,omitempty"`
	SubmittedBy *int             `json:"submitted_by,omitempty"`
	ApprovedBy  *int             `json:"approved_by,omitempty"`
	ApprovedAt  *time.Time       `json:"approved_at,omitempty"`
	LineItems   []OrderLineItem  `json:"line_items,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type OrderLineItem struct {
	ID            int     `json:"id"`
	OrderID       int     `json:"order_id"`
	Description   string  `json:"description"`
	Quantity      float64 `json:"quantity"`
	UnitPrice     float64 `json:"unit_price"`
	TotalPrice    float64 `json:"total_price"`
	WarehouseCode *string `json:"warehouse_code,omitempty"`
	TransportCode *string `json:"transport_code,omitempty"`
}

// ---- Invoices ----

type Invoice struct {
	ID              int        `json:"id"`
	InvoiceNumber   string     `json:"invoice_number"`
	OrderID         *int       `json:"order_id,omitempty"`
	VendorID        int        `json:"vendor_id"`
	VendorName      string     `json:"vendor_name,omitempty"`
	Status          string     `json:"status"`
	InvoiceAmount   float64    `json:"invoice_amount"`
	OrderAmount     *float64   `json:"order_amount,omitempty"`
	VarianceAmount  *float64   `json:"variance_amount,omitempty"`
	VariancePct     *float64   `json:"variance_pct,omitempty"`
	Currency        string     `json:"currency"`
	InvoiceDate     string     `json:"invoice_date"`
	DueDate         *string    `json:"due_date,omitempty"`
	Notes           *string    `json:"notes,omitempty"`
	MatchedBy       *int       `json:"matched_by,omitempty"`
	MatchedAt       *time.Time `json:"matched_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ---- Ledger ----

type LedgerEntry struct {
	ID            int       `json:"id"`
	EntryType     string    `json:"entry_type"` // AR, AP
	ReferenceType string    `json:"reference_type"`
	ReferenceID   int       `json:"reference_id"`
	VendorID      int       `json:"vendor_id"`
	VendorName    string    `json:"vendor_name,omitempty"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	Department    *string   `json:"department,omitempty"`
	CostCenter    *string   `json:"cost_center,omitempty"`
	Description   *string   `json:"description,omitempty"`
	PostedBy      *int      `json:"posted_by,omitempty"`
	PostedAt      time.Time `json:"posted_at"`
	CreatedAt     time.Time `json:"created_at"`
}

// ---- Settlements ----

type Settlement struct {
	ID              int        `json:"id"`
	VendorID        int        `json:"vendor_id"`
	VendorName      string     `json:"vendor_name,omitempty"`
	Status          string     `json:"status"`
	ARTotal         float64    `json:"ar_total"`
	APTotal         float64    `json:"ap_total"`
	NetAmount       float64    `json:"net_amount"`
	VarianceAmount  float64    `json:"variance_amount"`
	WriteoffAmount  float64    `json:"writeoff_amount"`
	Department      *string    `json:"department,omitempty"`
	CostCenter      *string    `json:"cost_center,omitempty"`
	PeriodStart     *string    `json:"period_start,omitempty"`
	PeriodEnd       *string    `json:"period_end,omitempty"`
	Notes           *string    `json:"notes,omitempty"`
	CreatedBy       *int       `json:"created_by,omitempty"`
	ApprovedBy      *int       `json:"approved_by,omitempty"`
	ApprovedAt      *time.Time `json:"approved_at,omitempty"`
	SettledAt       *time.Time `json:"settled_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type BillingRule struct {
	ID            int     `json:"id"`
	RuleType      string  `json:"rule_type"`
	Code          string  `json:"code"`
	Description   *string `json:"description,omitempty"`
	RatePerUnit   float64 `json:"rate_per_unit"`
	MinCharge     float64 `json:"min_charge"`
	MaxCharge     *float64 `json:"max_charge,omitempty"`
	EffectiveFrom string  `json:"effective_from"`
	EffectiveTo   *string `json:"effective_to,omitempty"`
	IsActive      bool    `json:"is_active"`
}

// ---- Reviews & Disputes ----

type VendorReview struct {
	ID             int              `json:"id"`
	VendorID       int              `json:"vendor_id"`
	VendorName     string           `json:"vendor_name,omitempty"`
	OrderID        *int             `json:"order_id,omitempty"`
	ReviewerID     int              `json:"reviewer_id"`
	Rating         int              `json:"rating"`
	Title          *string          `json:"title,omitempty"`
	Body           string           `json:"body"`
	ImageURLs      []string         `json:"image_urls,omitempty"`
	ReviewStatus   string           `json:"review_status"`
	DisclaimerText *string          `json:"disclaimer_text,omitempty"`
	Replies        []MerchantReply  `json:"replies,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type MerchantReply struct {
	ID        int       `json:"id"`
	ReviewID  int       `json:"review_id"`
	VendorID  int       `json:"vendor_id"`
	Body      string    `json:"body"`
	RepliedBy *int      `json:"replied_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Dispute struct {
	ID                   int        `json:"id"`
	ReviewID             int        `json:"review_id"`
	VendorID             int        `json:"vendor_id"`
	VendorName           string     `json:"vendor_name,omitempty"`
	Status               string     `json:"status"`
	Reason               string     `json:"reason"`
	EvidenceURLs         []string   `json:"evidence_urls,omitempty"`
	EvidenceMetadataEnc  []byte     `json:"-"`
	EvidenceMetadata     *string    `json:"evidence_metadata,omitempty"`
	MerchantResponse     *string    `json:"merchant_response,omitempty"`
	ArbitrationNotes     *string    `json:"arbitration_notes,omitempty"`
	ArbitrationOutcome   *string    `json:"arbitration_outcome,omitempty"`
	ArbitratedBy         *int       `json:"arbitrated_by,omitempty"`
	ArbitratedAt         *time.Time `json:"arbitrated_at,omitempty"`
	CreatedBy            int        `json:"created_by"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// MaskedReview is a DTO that hides reviewer identity for non-admin views.
type MaskedReview struct {
	ID             int              `json:"id"`
	VendorID       int              `json:"vendor_id"`
	VendorName     string           `json:"vendor_name,omitempty"`
	OrderID        *int             `json:"order_id,omitempty"`
	Rating         int              `json:"rating"`
	Title          *string          `json:"title,omitempty"`
	Body           string           `json:"body"`
	ImageURLs      []string         `json:"image_urls,omitempty"`
	ReviewStatus   string           `json:"review_status"`
	DisclaimerText *string          `json:"disclaimer_text,omitempty"`
	Replies        []MerchantReply  `json:"replies,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// MaskReview strips reviewer_id and other PII from a review for non-admin views.
func MaskReview(r VendorReview) MaskedReview {
	return MaskedReview{
		ID:             r.ID,
		VendorID:       r.VendorID,
		VendorName:     r.VendorName,
		OrderID:        r.OrderID,
		Rating:         r.Rating,
		Title:          r.Title,
		Body:           r.Body,
		ImageURLs:      r.ImageURLs,
		ReviewStatus:   r.ReviewStatus,
		DisclaimerText: r.DisclaimerText,
		Replies:        r.Replies,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

// MaskReviews applies MaskReview to a slice of reviews.
func MaskReviews(reviews []VendorReview) []MaskedReview {
	masked := make([]MaskedReview, len(reviews))
	for i, r := range reviews {
		masked[i] = MaskReview(r)
	}
	return masked
}

// MaskedDispute is a DTO that hides user-linked fields (created_by, arbitrated_by)
// for non-admin views.
type MaskedDispute struct {
	ID                 int        `json:"id"`
	ReviewID           int        `json:"review_id"`
	VendorID           int        `json:"vendor_id"`
	VendorName         string     `json:"vendor_name,omitempty"`
	Status             string     `json:"status"`
	Reason             string     `json:"reason"`
	EvidenceURLs       []string   `json:"evidence_urls,omitempty"`
	MerchantResponse   *string    `json:"merchant_response,omitempty"`
	ArbitrationNotes   *string    `json:"arbitration_notes,omitempty"`
	ArbitrationOutcome *string    `json:"arbitration_outcome,omitempty"`
	ArbitratedAt       *time.Time `json:"arbitrated_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// MaskDispute strips user-linked PII (created_by, arbitrated_by, evidence_metadata)
// from a dispute for non-admin views.
func MaskDispute(d Dispute) MaskedDispute {
	return MaskedDispute{
		ID:                 d.ID,
		ReviewID:           d.ReviewID,
		VendorID:           d.VendorID,
		VendorName:         d.VendorName,
		Status:             d.Status,
		Reason:             d.Reason,
		EvidenceURLs:       d.EvidenceURLs,
		MerchantResponse:   d.MerchantResponse,
		ArbitrationNotes:   d.ArbitrationNotes,
		ArbitrationOutcome: d.ArbitrationOutcome,
		ArbitratedAt:       d.ArbitratedAt,
		CreatedAt:          d.CreatedAt,
		UpdatedAt:          d.UpdatedAt,
	}
}

// MaskDisputes applies MaskDispute to a slice of disputes.
func MaskDisputes(disputes []Dispute) []MaskedDispute {
	masked := make([]MaskedDispute, len(disputes))
	for i, d := range disputes {
		masked[i] = MaskDispute(d)
	}
	return masked
}

// ---- Requests ----

type MatchInvoiceRequest struct {
	InvoiceID int `json:"invoice_id" validate:"required"`
	OrderID   int `json:"order_id" validate:"required"`
}

type CompareStatementRequest struct {
	VendorID    int     `json:"vendor_id" validate:"required"`
	StatementTotal float64 `json:"statement_total" validate:"required"`
	PeriodStart string  `json:"period_start"`
	PeriodEnd   string  `json:"period_end"`
}

type CompareStatementResponse struct {
	VendorID       int     `json:"vendor_id"`
	StatementTotal float64 `json:"statement_total"`
	LedgerTotal    float64 `json:"ledger_total"`
	Variance       float64 `json:"variance"`
	VariancePct    float64 `json:"variance_pct"`
	AutoWriteoff   bool    `json:"auto_writeoff"`
	SuggestedState string  `json:"suggested_state"`
}

type SettlementTransitionRequest struct {
	SettlementID int    `json:"settlement_id" validate:"required"`
	Action       string `json:"action" validate:"required"` // approve_writeoff, settle, dispute
	Notes        string `json:"notes,omitempty"`
}

type DisputeTransitionRequest struct {
	DisputeID          int      `json:"dispute_id" validate:"required"`
	Action             string   `json:"action" validate:"required"`
	EvidenceURLs       []string `json:"evidence_urls,omitempty"`
	EvidenceMetadata   string   `json:"evidence_metadata,omitempty"`
	MerchantResponse   string   `json:"merchant_response,omitempty"`
	ArbitrationNotes   string   `json:"arbitration_notes,omitempty"`
	ArbitrationOutcome string   `json:"arbitration_outcome,omitempty"` // hide, disclaimer, restore
}

type CostAllocationReport struct {
	Department  string  `json:"department"`
	CostCenter  string  `json:"cost_center"`
	ARTotal     float64 `json:"ar_total"`
	APTotal     float64 `json:"ap_total"`
	NetAmount   float64 `json:"net_amount"`
	EntryCount  int     `json:"entry_count"`
}

// ---- Create Requests ----

type CreateReviewRequest struct {
	VendorID  int      `json:"vendor_id" validate:"required"`
	OrderID   *int     `json:"order_id,omitempty"`
	Rating    int      `json:"rating" validate:"required"`
	Title     *string  `json:"title,omitempty"`
	Body      string   `json:"body" validate:"required"`
	ImageURLs []string `json:"image_urls,omitempty"`
}

type CreateMerchantReplyRequest struct {
	ReviewID int    `json:"review_id" validate:"required"`
	Body     string `json:"body" validate:"required"`
}

type CreateDisputeRequest struct {
	ReviewID int    `json:"review_id" validate:"required"`
	VendorID int    `json:"vendor_id" validate:"required"`
	Reason   string `json:"reason" validate:"required"`
}

type CreateLedgerEntryRequest struct {
	EntryType     string  `json:"entry_type" validate:"required"` // AR or AP
	ReferenceType string  `json:"reference_type" validate:"required"`
	ReferenceID   int     `json:"reference_id" validate:"required"`
	VendorID      int     `json:"vendor_id" validate:"required"`
	Amount        float64 `json:"amount" validate:"required"`
	Currency      string  `json:"currency,omitempty"`
	Department    *string `json:"department,omitempty"`
	CostCenter    *string `json:"cost_center,omitempty"`
	Description   *string `json:"description,omitempty"`
}

type CreateSettlementRequest struct {
	VendorID    int     `json:"vendor_id" validate:"required"`
	ARTotal     float64 `json:"ar_total"`
	APTotal     float64 `json:"ap_total"`
	Department  *string `json:"department,omitempty"`
	CostCenter  *string `json:"cost_center,omitempty"`
	PeriodStart *string `json:"period_start,omitempty"`
	PeriodEnd   *string `json:"period_end,omitempty"`
	Notes       *string `json:"notes,omitempty"`
}
