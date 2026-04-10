package repository

import (
	"context"
	"fmt"

	"wlpr-portal/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProcurementRepository struct {
	db *pgxpool.Pool
}

func NewProcurementRepository(db *pgxpool.Pool) *ProcurementRepository {
	return &ProcurementRepository{db: db}
}

// ---- Orders ----

func (r *ProcurementRepository) GetOrders(ctx context.Context, status string) ([]models.ProcurementOrder, error) {
	query := `
		SELECT o.id, o.order_number, o.vendor_id, v.name, o.status,
			   o.department, o.cost_center, o.total_amount, o.currency,
			   o.description, o.submitted_by, o.approved_by, o.approved_at,
			   o.created_at, o.updated_at
		FROM procurement_orders o
		JOIN vendors v ON v.id = o.vendor_id
	`
	args := []interface{}{}
	if status != "" {
		query += " WHERE o.status = $1"
		args = append(args, status)
	}
	query += " ORDER BY o.created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]models.ProcurementOrder, 0)
	for rows.Next() {
		var o models.ProcurementOrder
		if err := rows.Scan(
			&o.ID, &o.OrderNumber, &o.VendorID, &o.VendorName, &o.Status,
			&o.Department, &o.CostCenter, &o.TotalAmount, &o.Currency,
			&o.Description, &o.SubmittedBy, &o.ApprovedBy, &o.ApprovedAt,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func (r *ProcurementRepository) GetOrderByID(ctx context.Context, id int) (*models.ProcurementOrder, error) {
	o := &models.ProcurementOrder{}
	err := r.db.QueryRow(ctx, `
		SELECT o.id, o.order_number, o.vendor_id, v.name, o.status,
			   o.department, o.cost_center, o.total_amount, o.currency,
			   o.description, o.submitted_by, o.approved_by, o.approved_at,
			   o.created_at, o.updated_at
		FROM procurement_orders o
		JOIN vendors v ON v.id = o.vendor_id
		WHERE o.id = $1
	`, id).Scan(
		&o.ID, &o.OrderNumber, &o.VendorID, &o.VendorName, &o.Status,
		&o.Department, &o.CostCenter, &o.TotalAmount, &o.Currency,
		&o.Description, &o.SubmittedBy, &o.ApprovedBy, &o.ApprovedAt,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	items, err := r.GetOrderLineItems(ctx, id)
	if err == nil {
		o.LineItems = items
	}
	return o, nil
}

func (r *ProcurementRepository) GetOrderLineItems(ctx context.Context, orderID int) ([]models.OrderLineItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, description, quantity, unit_price, total_price,
			   warehouse_code, transport_code
		FROM order_line_items WHERE order_id = $1 ORDER BY id
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.OrderLineItem, 0)
	for rows.Next() {
		var li models.OrderLineItem
		if err := rows.Scan(&li.ID, &li.OrderID, &li.Description, &li.Quantity,
			&li.UnitPrice, &li.TotalPrice, &li.WarehouseCode, &li.TransportCode); err != nil {
			return nil, err
		}
		items = append(items, li)
	}
	return items, rows.Err()
}

func (r *ProcurementRepository) UpdateOrderStatus(ctx context.Context, id int, status string, approvedBy *int) error {
	if approvedBy != nil {
		_, err := r.db.Exec(ctx, `
			UPDATE procurement_orders SET status = $2, approved_by = $3, approved_at = NOW(), updated_at = NOW()
			WHERE id = $1
		`, id, status, *approvedBy)
		return err
	}
	_, err := r.db.Exec(ctx, `
		UPDATE procurement_orders SET status = $2, updated_at = NOW() WHERE id = $1
	`, id, status)
	return err
}

// ---- Invoices ----

func (r *ProcurementRepository) GetInvoices(ctx context.Context, status string) ([]models.Invoice, error) {
	query := `
		SELECT i.id, i.invoice_number, i.order_id, i.vendor_id, v.name,
			   i.status, i.invoice_amount, i.order_amount, i.variance_amount,
			   i.variance_pct, i.currency, i.invoice_date::text, i.due_date::text,
			   i.notes, i.matched_by, i.matched_at, i.created_at, i.updated_at
		FROM invoices i
		JOIN vendors v ON v.id = i.vendor_id
	`
	args := []interface{}{}
	if status != "" {
		query += " WHERE i.status = $1"
		args = append(args, status)
	}
	query += " ORDER BY i.created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invoices := make([]models.Invoice, 0)
	for rows.Next() {
		var inv models.Invoice
		if err := rows.Scan(
			&inv.ID, &inv.InvoiceNumber, &inv.OrderID, &inv.VendorID, &inv.VendorName,
			&inv.Status, &inv.InvoiceAmount, &inv.OrderAmount, &inv.VarianceAmount,
			&inv.VariancePct, &inv.Currency, &inv.InvoiceDate, &inv.DueDate,
			&inv.Notes, &inv.MatchedBy, &inv.MatchedAt, &inv.CreatedAt, &inv.UpdatedAt,
		); err != nil {
			return nil, err
		}
		invoices = append(invoices, inv)
	}
	return invoices, rows.Err()
}

func (r *ProcurementRepository) GetInvoiceByID(ctx context.Context, id int) (*models.Invoice, error) {
	inv := &models.Invoice{}
	err := r.db.QueryRow(ctx, `
		SELECT i.id, i.invoice_number, i.order_id, i.vendor_id, v.name,
			   i.status, i.invoice_amount, i.order_amount, i.variance_amount,
			   i.variance_pct, i.currency, i.invoice_date::text, i.due_date::text,
			   i.notes, i.matched_by, i.matched_at, i.created_at, i.updated_at
		FROM invoices i
		JOIN vendors v ON v.id = i.vendor_id
		WHERE i.id = $1
	`, id).Scan(
		&inv.ID, &inv.InvoiceNumber, &inv.OrderID, &inv.VendorID, &inv.VendorName,
		&inv.Status, &inv.InvoiceAmount, &inv.OrderAmount, &inv.VarianceAmount,
		&inv.VariancePct, &inv.Currency, &inv.InvoiceDate, &inv.DueDate,
		&inv.Notes, &inv.MatchedBy, &inv.MatchedAt, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return inv, err
}

func (r *ProcurementRepository) MatchInvoice(ctx context.Context, invoiceID, orderID, matchedBy int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE invoices SET
			order_id = $2,
			order_amount = (SELECT total_amount FROM procurement_orders WHERE id = $2),
			variance_amount = invoice_amount - (SELECT total_amount FROM procurement_orders WHERE id = $2),
			variance_pct = CASE
				WHEN (SELECT total_amount FROM procurement_orders WHERE id = $2) = 0 THEN 0
				ELSE ROUND(((invoice_amount - (SELECT total_amount FROM procurement_orders WHERE id = $2))
					/ (SELECT total_amount FROM procurement_orders WHERE id = $2)) * 100, 2)
			END,
			status = CASE
				WHEN invoice_amount = (SELECT total_amount FROM procurement_orders WHERE id = $2)
				THEN 'matched'
				ELSE 'variance_detected'
			END,
			matched_by = $3,
			matched_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, invoiceID, orderID, matchedBy)
	return err
}

func (r *ProcurementRepository) UpdateInvoiceStatus(ctx context.Context, id int, status string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE invoices SET status = $2, updated_at = NOW() WHERE id = $1
	`, id, status)
	return err
}

// ---- Ledger ----

func (r *ProcurementRepository) GetLedgerEntries(ctx context.Context, vendorID int, entryType string) ([]models.LedgerEntry, error) {
	query := `
		SELECT le.id, le.entry_type, le.reference_type, le.reference_id,
			   le.vendor_id, v.name, le.amount, le.currency,
			   le.department, le.cost_center, le.description,
			   le.posted_by, le.posted_at, le.created_at
		FROM ledger_entries le
		JOIN vendors v ON v.id = le.vendor_id
		WHERE 1=1
	`
	args := []interface{}{}
	idx := 1

	if vendorID > 0 {
		query += fmt.Sprintf(" AND le.vendor_id = $%d", idx)
		args = append(args, vendorID)
		idx++
	}
	if entryType != "" {
		query += fmt.Sprintf(" AND le.entry_type = $%d", idx)
		args = append(args, entryType)
		idx++
	}
	query += " ORDER BY le.posted_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]models.LedgerEntry, 0)
	for rows.Next() {
		var le models.LedgerEntry
		if err := rows.Scan(
			&le.ID, &le.EntryType, &le.ReferenceType, &le.ReferenceID,
			&le.VendorID, &le.VendorName, &le.Amount, &le.Currency,
			&le.Department, &le.CostCenter, &le.Description,
			&le.PostedBy, &le.PostedAt, &le.CreatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, le)
	}
	return entries, rows.Err()
}

func (r *ProcurementRepository) CreateLedgerEntry(ctx context.Context, le models.LedgerEntry) (int, error) {
	var id int
	err := r.db.QueryRow(ctx, `
		INSERT INTO ledger_entries (entry_type, reference_type, reference_id, vendor_id,
			amount, currency, department, cost_center, description, posted_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, le.EntryType, le.ReferenceType, le.ReferenceID, le.VendorID,
		le.Amount, le.Currency, le.Department, le.CostCenter,
		le.Description, le.PostedBy).Scan(&id)
	return id, err
}

// CompareStatement totals all AR entries for a vendor in a period and compares against a statement.
func (r *ProcurementRepository) SumLedgerByVendor(ctx context.Context, vendorID int, entryType, periodStart, periodEnd string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM ledger_entries
		WHERE vendor_id = $1 AND entry_type = $2
	`
	args := []interface{}{vendorID, entryType}
	idx := 3
	if periodStart != "" {
		query += fmt.Sprintf(" AND posted_at >= $%d", idx)
		args = append(args, periodStart)
		idx++
	}
	if periodEnd != "" {
		query += fmt.Sprintf(" AND posted_at <= $%d::date + INTERVAL '1 day'", idx)
		args = append(args, periodEnd)
		idx++
	}

	var total float64
	err := r.db.QueryRow(ctx, query, args...).Scan(&total)
	return total, err
}

// ---- Settlements ----

func (r *ProcurementRepository) GetSettlements(ctx context.Context, status string) ([]models.Settlement, error) {
	query := `
		SELECT s.id, s.vendor_id, v.name, s.status,
			   s.ar_total, s.ap_total, s.net_amount, s.variance_amount, s.writeoff_amount,
			   s.department, s.cost_center,
			   s.period_start::text, s.period_end::text,
			   s.notes, s.created_by, s.approved_by, s.approved_at, s.settled_at,
			   s.created_at, s.updated_at
		FROM settlements s
		JOIN vendors v ON v.id = s.vendor_id
	`
	args := []interface{}{}
	if status != "" {
		query += " WHERE s.status = $1"
		args = append(args, status)
	}
	query += " ORDER BY s.created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settlements := make([]models.Settlement, 0)
	for rows.Next() {
		var s models.Settlement
		if err := rows.Scan(
			&s.ID, &s.VendorID, &s.VendorName, &s.Status,
			&s.ARTotal, &s.APTotal, &s.NetAmount, &s.VarianceAmount, &s.WriteoffAmount,
			&s.Department, &s.CostCenter, &s.PeriodStart, &s.PeriodEnd,
			&s.Notes, &s.CreatedBy, &s.ApprovedBy, &s.ApprovedAt, &s.SettledAt,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		settlements = append(settlements, s)
	}
	return settlements, rows.Err()
}

func (r *ProcurementRepository) GetSettlementByID(ctx context.Context, id int) (*models.Settlement, error) {
	s := &models.Settlement{}
	err := r.db.QueryRow(ctx, `
		SELECT s.id, s.vendor_id, v.name, s.status,
			   s.ar_total, s.ap_total, s.net_amount, s.variance_amount, s.writeoff_amount,
			   s.department, s.cost_center,
			   s.period_start::text, s.period_end::text,
			   s.notes, s.created_by, s.approved_by, s.approved_at, s.settled_at,
			   s.created_at, s.updated_at
		FROM settlements s
		JOIN vendors v ON v.id = s.vendor_id
		WHERE s.id = $1
	`, id).Scan(
		&s.ID, &s.VendorID, &s.VendorName, &s.Status,
		&s.ARTotal, &s.APTotal, &s.NetAmount, &s.VarianceAmount, &s.WriteoffAmount,
		&s.Department, &s.CostCenter, &s.PeriodStart, &s.PeriodEnd,
		&s.Notes, &s.CreatedBy, &s.ApprovedBy, &s.ApprovedAt, &s.SettledAt,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return s, err
}

func (r *ProcurementRepository) UpdateSettlement(ctx context.Context, id int, status string, approvedBy *int, notes string) error {
	query := "UPDATE settlements SET status = $2, notes = $3, updated_at = NOW()"
	args := []interface{}{id, status, notes}
	if approvedBy != nil {
		query += ", approved_by = $4, approved_at = NOW()"
		args = append(args, *approvedBy)
	}
	if status == "settled" {
		query += ", settled_at = NOW()"
	}
	if status == "writeoff_approved" {
		query += ", writeoff_amount = variance_amount"
	}
	query += " WHERE id = $1"
	_, err := r.db.Exec(ctx, query, args...)
	return err
}

// ---- Cost Allocation ----

func (r *ProcurementRepository) GetCostAllocation(ctx context.Context) ([]models.CostAllocationReport, error) {
	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(department, 'Unassigned'), COALESCE(cost_center, 'N/A'),
			   COALESCE(SUM(CASE WHEN entry_type = 'AR' THEN amount ELSE 0 END), 0) AS ar_total,
			   COALESCE(SUM(CASE WHEN entry_type = 'AP' THEN amount ELSE 0 END), 0) AS ap_total,
			   COALESCE(SUM(CASE WHEN entry_type = 'AR' THEN amount ELSE -amount END), 0) AS net,
			   COUNT(*) AS entry_count
		FROM ledger_entries
		GROUP BY department, cost_center
		ORDER BY department, cost_center
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := make([]models.CostAllocationReport, 0)
	for rows.Next() {
		var r models.CostAllocationReport
		if err := rows.Scan(&r.Department, &r.CostCenter, &r.ARTotal, &r.APTotal, &r.NetAmount, &r.EntryCount); err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

// ---- Billing Rules ----

func (r *ProcurementRepository) GetBillingRules(ctx context.Context, ruleType string) ([]models.BillingRule, error) {
	query := "SELECT id, rule_type, code, description, rate_per_unit, min_charge, max_charge, effective_from::text, effective_to::text, is_active FROM billing_rules"
	args := []interface{}{}
	if ruleType != "" {
		query += " WHERE rule_type = $1"
		args = append(args, ruleType)
	}
	query += " ORDER BY rule_type, code"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]models.BillingRule, 0)
	for rows.Next() {
		var br models.BillingRule
		if err := rows.Scan(&br.ID, &br.RuleType, &br.Code, &br.Description, &br.RatePerUnit,
			&br.MinCharge, &br.MaxCharge, &br.EffectiveFrom, &br.EffectiveTo, &br.IsActive); err != nil {
			return nil, err
		}
		rules = append(rules, br)
	}
	return rules, rows.Err()
}

// ---- Vendors ----

func (r *ProcurementRepository) GetVendors(ctx context.Context) ([]models.Vendor, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, code, contact_email, contact_phone, address, is_active, created_at, updated_at
		FROM vendors WHERE is_active = TRUE ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	vendors := make([]models.Vendor, 0)
	for rows.Next() {
		var v models.Vendor
		if err := rows.Scan(&v.ID, &v.Name, &v.Code, &v.ContactEmail, &v.ContactPhone,
			&v.Address, &v.IsActive, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		vendors = append(vendors, v)
	}
	return vendors, rows.Err()
}

// ---- Reviews ----

func (r *ProcurementRepository) GetReviews(ctx context.Context, vendorID int) ([]models.VendorReview, error) {
	query := `
		SELECT vr.id, vr.vendor_id, v.name, vr.order_id, vr.reviewer_id,
			   vr.rating, vr.title, vr.body, vr.image_urls, vr.review_status,
			   vr.disclaimer_text, vr.created_at, vr.updated_at
		FROM vendor_reviews vr
		JOIN vendors v ON v.id = vr.vendor_id
	`
	args := []interface{}{}
	if vendorID > 0 {
		query += " WHERE vr.vendor_id = $1"
		args = append(args, vendorID)
	}
	query += " ORDER BY vr.created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reviews := make([]models.VendorReview, 0)
	for rows.Next() {
		var rv models.VendorReview
		if err := rows.Scan(
			&rv.ID, &rv.VendorID, &rv.VendorName, &rv.OrderID, &rv.ReviewerID,
			&rv.Rating, &rv.Title, &rv.Body, &rv.ImageURLs, &rv.ReviewStatus,
			&rv.DisclaimerText, &rv.CreatedAt, &rv.UpdatedAt,
		); err != nil {
			return nil, err
		}
		reviews = append(reviews, rv)
	}
	return reviews, rows.Err()
}

func (r *ProcurementRepository) UpdateReviewStatus(ctx context.Context, id int, status, disclaimerText string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE vendor_reviews SET review_status = $2, disclaimer_text = $3, updated_at = NOW()
		WHERE id = $1
	`, id, status, disclaimerText)
	return err
}

// ---- Disputes ----

func (r *ProcurementRepository) GetDisputes(ctx context.Context, status string) ([]models.Dispute, error) {
	query := `
		SELECT d.id, d.review_id, d.vendor_id, v.name, d.status, d.reason,
			   d.evidence_urls, d.evidence_metadata_enc, d.merchant_response, d.arbitration_notes,
			   d.arbitration_outcome, d.arbitrated_by, d.arbitrated_at,
			   d.created_by, d.created_at, d.updated_at
		FROM disputes d
		JOIN vendors v ON v.id = d.vendor_id
	`
	args := []interface{}{}
	if status != "" {
		query += " WHERE d.status = $1"
		args = append(args, status)
	}
	query += " ORDER BY d.created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	disputes := make([]models.Dispute, 0)
	for rows.Next() {
		var d models.Dispute
		if err := rows.Scan(
			&d.ID, &d.ReviewID, &d.VendorID, &d.VendorName, &d.Status, &d.Reason,
			&d.EvidenceURLs, &d.EvidenceMetadataEnc, &d.MerchantResponse, &d.ArbitrationNotes,
			&d.ArbitrationOutcome, &d.ArbitratedBy, &d.ArbitratedAt,
			&d.CreatedBy, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		disputes = append(disputes, d)
	}
	return disputes, rows.Err()
}

func (r *ProcurementRepository) GetDisputeByID(ctx context.Context, id int) (*models.Dispute, error) {
	d := &models.Dispute{}
	err := r.db.QueryRow(ctx, `
		SELECT d.id, d.review_id, d.vendor_id, v.name, d.status, d.reason,
			   d.evidence_urls, d.evidence_metadata_enc, d.merchant_response, d.arbitration_notes,
			   d.arbitration_outcome, d.arbitrated_by, d.arbitrated_at,
			   d.created_by, d.created_at, d.updated_at
		FROM disputes d
		JOIN vendors v ON v.id = d.vendor_id
		WHERE d.id = $1
	`, id).Scan(
		&d.ID, &d.ReviewID, &d.VendorID, &d.VendorName, &d.Status, &d.Reason,
		&d.EvidenceURLs, &d.EvidenceMetadataEnc, &d.MerchantResponse, &d.ArbitrationNotes,
		&d.ArbitrationOutcome, &d.ArbitratedBy, &d.ArbitratedAt,
		&d.CreatedBy, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return d, err
}

// ---- Create Operations ----

func (r *ProcurementRepository) CreateReview(ctx context.Context, review models.CreateReviewRequest, reviewerID int) (*models.VendorReview, error) {
	rv := &models.VendorReview{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO vendor_reviews (vendor_id, order_id, reviewer_id, rating, title, body, image_urls)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, vendor_id, order_id, reviewer_id, rating, title, body, image_urls,
				  review_status, disclaimer_text, created_at, updated_at
	`, review.VendorID, review.OrderID, reviewerID, review.Rating, review.Title,
		review.Body, review.ImageURLs).Scan(
		&rv.ID, &rv.VendorID, &rv.OrderID, &rv.ReviewerID, &rv.Rating, &rv.Title,
		&rv.Body, &rv.ImageURLs, &rv.ReviewStatus, &rv.DisclaimerText,
		&rv.CreatedAt, &rv.UpdatedAt,
	)
	return rv, err
}

func (r *ProcurementRepository) CreateMerchantReply(ctx context.Context, reviewID, vendorID, repliedBy int, body string) (*models.MerchantReply, error) {
	reply := &models.MerchantReply{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO merchant_replies (review_id, vendor_id, body, replied_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, review_id, vendor_id, body, replied_by, created_at
	`, reviewID, vendorID, body, repliedBy).Scan(
		&reply.ID, &reply.ReviewID, &reply.VendorID, &reply.Body, &reply.RepliedBy, &reply.CreatedAt,
	)
	return reply, err
}

func (r *ProcurementRepository) CreateDispute(ctx context.Context, reviewID, vendorID, createdBy int, reason string) (*models.Dispute, error) {
	d := &models.Dispute{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO disputes (review_id, vendor_id, status, reason, created_by)
		VALUES ($1, $2, 'created', $3, $4)
		RETURNING id, review_id, vendor_id, status, reason, evidence_urls,
				  merchant_response, arbitration_notes, arbitration_outcome,
				  arbitrated_by, arbitrated_at, created_by, created_at, updated_at
	`, reviewID, vendorID, reason, createdBy).Scan(
		&d.ID, &d.ReviewID, &d.VendorID, &d.Status, &d.Reason, &d.EvidenceURLs,
		&d.MerchantResponse, &d.ArbitrationNotes, &d.ArbitrationOutcome,
		&d.ArbitratedBy, &d.ArbitratedAt, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt,
	)
	return d, err
}

func (r *ProcurementRepository) CreateSettlement(ctx context.Context, req models.CreateSettlementRequest, createdBy int) (*models.Settlement, error) {
	netAmount := req.ARTotal - req.APTotal
	varianceAmount := req.ARTotal - req.APTotal
	s := &models.Settlement{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO settlements (vendor_id, status, ar_total, ap_total, net_amount, variance_amount,
			department, cost_center, period_start, period_end, notes, created_by)
		VALUES ($1, 'open', $2, $3, $4, $5, $6, $7, $8::date, $9::date, $10, $11)
		RETURNING id, vendor_id, status, ar_total, ap_total, net_amount, variance_amount,
				  writeoff_amount, department, cost_center,
				  period_start::text, period_end::text,
				  notes, created_by, approved_by, approved_at, settled_at,
				  created_at, updated_at
	`, req.VendorID, req.ARTotal, req.APTotal, netAmount, varianceAmount,
		req.Department, req.CostCenter, req.PeriodStart, req.PeriodEnd, req.Notes, createdBy).Scan(
		&s.ID, &s.VendorID, &s.Status, &s.ARTotal, &s.APTotal, &s.NetAmount,
		&s.VarianceAmount, &s.WriteoffAmount, &s.Department, &s.CostCenter,
		&s.PeriodStart, &s.PeriodEnd, &s.Notes, &s.CreatedBy, &s.ApprovedBy,
		&s.ApprovedAt, &s.SettledAt, &s.CreatedAt, &s.UpdatedAt,
	)
	return s, err
}

// TransitionDispute updates the dispute status. The DB trigger trg_dispute_state_machine
// enforces valid transitions and will raise an exception for invalid ones.
func (r *ProcurementRepository) TransitionDispute(ctx context.Context, id int, newStatus string, updates map[string]interface{}) error {
	query := "UPDATE disputes SET status = $2, updated_at = NOW()"
	args := []interface{}{id, newStatus}
	idx := 3

	if v, ok := updates["evidence_urls"]; ok {
		query += fmt.Sprintf(", evidence_urls = $%d", idx)
		args = append(args, v)
		idx++
	}
	if v, ok := updates["evidence_metadata_enc"]; ok {
		query += fmt.Sprintf(", evidence_metadata_enc = $%d", idx)
		args = append(args, v)
		idx++
	}
	if v, ok := updates["merchant_response"]; ok {
		query += fmt.Sprintf(", merchant_response = $%d", idx)
		args = append(args, v)
		idx++
	}
	if v, ok := updates["arbitration_notes"]; ok {
		query += fmt.Sprintf(", arbitration_notes = $%d", idx)
		args = append(args, v)
		idx++
	}
	if v, ok := updates["arbitration_outcome"]; ok {
		query += fmt.Sprintf(", arbitration_outcome = $%d", idx)
		args = append(args, v)
		idx++
	}
	if v, ok := updates["arbitrated_by"]; ok {
		query += fmt.Sprintf(", arbitrated_by = $%d, arbitrated_at = NOW()", idx)
		args = append(args, v)
		idx++
	}

	query += " WHERE id = $1"
	_, err := r.db.Exec(ctx, query, args...)
	return err
}

// CreateVendor inserts a new vendor.
func (r *ProcurementRepository) CreateVendor(ctx context.Context, v models.Vendor) (*models.Vendor, error) {
	var id int
	err := r.db.QueryRow(ctx, `
		INSERT INTO vendors (name, code, contact_email, contact_phone, address, is_active)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		v.Name, v.Code, v.ContactEmail, v.ContactPhone, v.Address, true,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert vendor: %w", err)
	}
	v.ID = id
	v.IsActive = true
	return &v, nil
}

// CreateOrder inserts a new procurement order.
func (r *ProcurementRepository) CreateOrder(ctx context.Context, o models.ProcurementOrder, submittedBy int) (*models.ProcurementOrder, error) {
	var id int
	err := r.db.QueryRow(ctx, `
		INSERT INTO procurement_orders (order_number, vendor_id, status, department, cost_center,
			total_amount, currency, description, submitted_by)
		VALUES ($1, $2, 'submitted', $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		o.OrderNumber, o.VendorID, o.Department, o.CostCenter,
		o.TotalAmount, o.Currency, o.Description, submittedBy,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert order: %w", err)
	}
	o.ID = id
	o.Status = "submitted"
	return &o, nil
}

// CreateInvoice inserts a new invoice.
func (r *ProcurementRepository) CreateInvoice(ctx context.Context, inv models.Invoice) (*models.Invoice, error) {
	var id int
	err := r.db.QueryRow(ctx, `
		INSERT INTO invoices (invoice_number, vendor_id, status, invoice_amount, currency,
			invoice_date, due_date, notes)
		VALUES ($1, $2, 'pending', $3, $4, $5, $6, $7)
		RETURNING id`,
		inv.InvoiceNumber, inv.VendorID, inv.InvoiceAmount, inv.Currency,
		inv.InvoiceDate, inv.DueDate, inv.Notes,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert invoice: %w", err)
	}
	inv.ID = id
	inv.Status = "pending"
	return &inv, nil
}
