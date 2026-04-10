package handlers

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/repository"
	"wlpr-portal/internal/services"
	"wlpr-portal/pkg/crypto"

	"github.com/labstack/echo/v4"
)

// isAdmin checks whether the current user has system_admin or content_moderator role.
func isAdmin(c echo.Context) bool {
	roles, ok := c.Get("roles").([]string)
	if !ok {
		return false
	}
	for _, r := range roles {
		if r == "system_admin" || r == "content_moderator" {
			return true
		}
	}
	return false
}

// decryptDisputeMetadata decrypts evidence_metadata_enc and populates the EvidenceMetadata field.
// Only called for admin-role users; non-admin users never see the decrypted metadata.
func decryptDisputeMetadata(d *models.Dispute) {
	if len(d.EvidenceMetadataEnc) > 0 {
		plaintext, err := crypto.DecryptString(d.EvidenceMetadataEnc)
		if err != nil {
			log.Printf("[procurement] evidence metadata decryption error dispute=%d: %v", d.ID, err)
			return
		}
		d.EvidenceMetadata = &plaintext
	}
}

type ProcurementHandler struct {
	procRepo   *repository.ProcurementRepository
	reconSvc   *services.ReconciliationService
	exportSink *services.ExportSinkService
}

func NewProcurementHandler(procRepo *repository.ProcurementRepository, reconSvc *services.ReconciliationService, exportSink *services.ExportSinkService) *ProcurementHandler {
	return &ProcurementHandler{
		procRepo:   procRepo,
		reconSvc:   reconSvc,
		exportSink: exportSink,
	}
}

// ---- Vendors ----

// GET /api/procurement/vendors
func (h *ProcurementHandler) GetVendors(c echo.Context) error {
	vendors, err := h.procRepo.GetVendors(c.Request().Context())
	if err != nil {
		log.Printf("[procurement] get vendors error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve vendors")
	}
	return c.JSON(http.StatusOK, vendors)
}

// ---- Orders ----

// GET /api/procurement/orders?status=submitted
func (h *ProcurementHandler) GetOrders(c echo.Context) error {
	orders, err := h.procRepo.GetOrders(c.Request().Context(), c.QueryParam("status"))
	if err != nil {
		log.Printf("[procurement] get orders error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve orders")
	}
	return c.JSON(http.StatusOK, orders)
}

// GET /api/procurement/orders/:id
func (h *ProcurementHandler) GetOrder(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid order ID")
	}
	order, err := h.procRepo.GetOrderByID(c.Request().Context(), id)
	if err != nil || order == nil {
		return echo.NewHTTPError(http.StatusNotFound, "order not found")
	}
	return c.JSON(http.StatusOK, order)
}

// PUT /api/procurement/orders/:id/approve
func (h *ProcurementHandler) ApproveOrder(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid order ID")
	}
	userID := c.Get("user_id").(int)
	if err := h.procRepo.UpdateOrderStatus(c.Request().Context(), id, "approved", &userID); err != nil {
		log.Printf("[procurement] approve order error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to approve order")
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "order approved"})
}

// ---- Invoices ----

// GET /api/procurement/invoices?status=pending
func (h *ProcurementHandler) GetInvoices(c echo.Context) error {
	invoices, err := h.procRepo.GetInvoices(c.Request().Context(), c.QueryParam("status"))
	if err != nil {
		log.Printf("[procurement] get invoices error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve invoices")
	}
	return c.JSON(http.StatusOK, invoices)
}

// GET /api/procurement/invoices/:id
func (h *ProcurementHandler) GetInvoice(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid invoice ID")
	}
	inv, err := h.procRepo.GetInvoiceByID(c.Request().Context(), id)
	if err != nil || inv == nil {
		return echo.NewHTTPError(http.StatusNotFound, "invoice not found")
	}
	return c.JSON(http.StatusOK, inv)
}

// POST /api/procurement/invoices/match
func (h *ProcurementHandler) MatchInvoice(c echo.Context) error {
	var req models.MatchInvoiceRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.InvoiceID == 0 || req.OrderID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invoice_id and order_id are required")
	}
	userID := c.Get("user_id").(int)
	if err := h.reconSvc.MatchInvoice(c.Request().Context(), req.InvoiceID, req.OrderID, userID); err != nil {
		log.Printf("[procurement] match invoice error: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "invoice matching failed")
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "invoice matched"})
}

// ---- Reconciliation ----

// POST /api/procurement/reconciliation/compare
func (h *ProcurementHandler) CompareStatement(c echo.Context) error {
	var req models.CompareStatementRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.reconSvc.CompareStatement(c.Request().Context(), req)
	if err != nil {
		log.Printf("[procurement] compare statement error: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "statement comparison failed")
	}
	return c.JSON(http.StatusOK, resp)
}

// GET /api/procurement/ledger?vendor_id=1&type=AR
func (h *ProcurementHandler) GetLedger(c echo.Context) error {
	vendorID := 0
	if v := c.QueryParam("vendor_id"); v != "" {
		vendorID, _ = strconv.Atoi(v)
	}
	entries, err := h.procRepo.GetLedgerEntries(c.Request().Context(), vendorID, c.QueryParam("type"))
	if err != nil {
		log.Printf("[procurement] get ledger entries error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve ledger entries")
	}
	return c.JSON(http.StatusOK, entries)
}

// GET /api/procurement/cost-allocation
func (h *ProcurementHandler) GetCostAllocation(c echo.Context) error {
	report, err := h.procRepo.GetCostAllocation(c.Request().Context())
	if err != nil {
		log.Printf("[procurement] get cost allocation error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve cost allocation")
	}
	return c.JSON(http.StatusOK, report)
}

// GET /api/procurement/billing-rules?type=warehouse
func (h *ProcurementHandler) GetBillingRules(c echo.Context) error {
	rules, err := h.procRepo.GetBillingRules(c.Request().Context(), c.QueryParam("type"))
	if err != nil {
		log.Printf("[procurement] get billing rules error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve billing rules")
	}
	return c.JSON(http.StatusOK, rules)
}

// ---- Settlements ----

// GET /api/procurement/settlements?status=open
func (h *ProcurementHandler) GetSettlements(c echo.Context) error {
	settlements, err := h.procRepo.GetSettlements(c.Request().Context(), c.QueryParam("status"))
	if err != nil {
		log.Printf("[procurement] get settlements error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve settlements")
	}
	return c.JSON(http.StatusOK, settlements)
}

// POST /api/procurement/settlements/transition
func (h *ProcurementHandler) TransitionSettlement(c echo.Context) error {
	var req models.SettlementTransitionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	userID := c.Get("user_id").(int)
	if err := h.reconSvc.TransitionSettlement(c.Request().Context(), req, userID); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "invalid transition") {
			log.Printf("[procurement] transition settlement conflict: %v", err)
			return echo.NewHTTPError(http.StatusConflict, "invalid state transition")
		}
		log.Printf("[procurement] transition settlement error: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "settlement transition failed")
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "settlement updated"})
}

// GET /api/procurement/export/ledger?vendor_id=1
// Streams a CSV file download of ledger entries and also exports to configured sinks
// (offline file-drop and/or LAN webhook).
func (h *ProcurementHandler) ExportLedger(c echo.Context) error {
	vendorID := 0
	if v := c.QueryParam("vendor_id"); v != "" {
		vendorID, _ = strconv.Atoi(v)
	}

	entries, err := h.reconSvc.GetLedgerForExport(c.Request().Context(), vendorID)
	if err != nil {
		log.Printf("[procurement] export ledger error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to export ledger")
	}

	// Export to configured sinks (file-drop / LAN webhook) in the background
	if h.exportSink != nil {
		go func() {
			if err := h.exportSink.ExportLedgerToSinks(context.Background(), entries); err != nil {
				log.Printf("[procurement] sink export error: %v", err)
			}
		}()
	}

	c.Response().Header().Set("Content-Type", "text/csv")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=ledger_export.csv")
	c.Response().WriteHeader(http.StatusOK)

	writer := csv.NewWriter(c.Response().Writer)
	_ = writer.Write([]string{
		"Entry ID", "Type", "Reference Type", "Reference ID",
		"Vendor ID", "Vendor Name", "Amount", "Currency",
		"Department", "Cost Center", "Description", "Posted At",
	})

	for _, e := range entries {
		dept, cc, desc := "", "", ""
		if e.Department != nil {
			dept = *e.Department
		}
		if e.CostCenter != nil {
			cc = *e.CostCenter
		}
		if e.Description != nil {
			desc = *e.Description
		}
		_ = writer.Write([]string{
			fmt.Sprintf("%d", e.ID), e.EntryType, e.ReferenceType,
			fmt.Sprintf("%d", e.ReferenceID), fmt.Sprintf("%d", e.VendorID),
			e.VendorName, fmt.Sprintf("%.2f", e.Amount), e.Currency,
			dept, cc, desc, e.PostedAt.Format("2006-01-02 15:04:05"),
		})
	}

	writer.Flush()
	return nil
}

// GET /api/procurement/export/settlements
// Streams a CSV file download of settlement data and also exports to configured sinks.
func (h *ProcurementHandler) ExportSettlements(c echo.Context) error {
	settlements, err := h.reconSvc.GetSettlementsForExport(c.Request().Context())
	if err != nil {
		log.Printf("[procurement] export settlements error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to export settlements")
	}

	// Export to configured sinks (file-drop / LAN webhook) in the background
	if h.exportSink != nil {
		go func() {
			if err := h.exportSink.ExportSettlementsToSinks(context.Background(), settlements); err != nil {
				log.Printf("[procurement] sink export error: %v", err)
			}
		}()
	}

	c.Response().Header().Set("Content-Type", "text/csv")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=settlements_export.csv")
	c.Response().WriteHeader(http.StatusOK)

	writer := csv.NewWriter(c.Response().Writer)
	_ = writer.Write([]string{
		"Settlement ID", "Vendor ID", "Vendor Name", "Status",
		"AR Total", "AP Total", "Net Amount", "Variance",
		"Write-off", "Department", "Cost Center",
		"Period Start", "Period End",
	})

	for _, s := range settlements {
		dept, cc, ps, pe := "", "", "", ""
		if s.Department != nil {
			dept = *s.Department
		}
		if s.CostCenter != nil {
			cc = *s.CostCenter
		}
		if s.PeriodStart != nil {
			ps = *s.PeriodStart
		}
		if s.PeriodEnd != nil {
			pe = *s.PeriodEnd
		}
		_ = writer.Write([]string{
			fmt.Sprintf("%d", s.ID), fmt.Sprintf("%d", s.VendorID),
			s.VendorName, s.Status,
			fmt.Sprintf("%.2f", s.ARTotal), fmt.Sprintf("%.2f", s.APTotal),
			fmt.Sprintf("%.2f", s.NetAmount), fmt.Sprintf("%.2f", s.VarianceAmount),
			fmt.Sprintf("%.2f", s.WriteoffAmount),
			dept, cc, ps, pe,
		})
	}

	writer.Flush()
	return nil
}

// ---- Create: Ledger Entries (Finance) ----

// POST /api/procurement/ledger
func (h *ProcurementHandler) CreateLedgerEntry(c echo.Context) error {
	var req models.CreateLedgerEntryRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.EntryType != "AR" && req.EntryType != "AP" {
		return echo.NewHTTPError(http.StatusBadRequest, "entry_type must be AR or AP")
	}
	if req.VendorID == 0 || req.ReferenceID == 0 || req.Amount == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "vendor_id, reference_id, and amount are required")
	}

	userID := c.Get("user_id").(int)
	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}
	entry := models.LedgerEntry{
		EntryType:     req.EntryType,
		ReferenceType: req.ReferenceType,
		ReferenceID:   req.ReferenceID,
		VendorID:      req.VendorID,
		Amount:        req.Amount,
		Currency:      currency,
		Department:    req.Department,
		CostCenter:    req.CostCenter,
		Description:   req.Description,
		PostedBy:      &userID,
	}
	id, err := h.procRepo.CreateLedgerEntry(c.Request().Context(), entry)
	if err != nil {
		log.Printf("[procurement] create ledger entry error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create ledger entry")
	}
	return c.JSON(http.StatusCreated, map[string]interface{}{"message": "ledger entry created", "id": id})
}

// ---- Create: Settlements (Finance) ----

// POST /api/procurement/settlements
func (h *ProcurementHandler) CreateSettlement(c echo.Context) error {
	var req models.CreateSettlementRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.VendorID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "vendor_id is required")
	}

	userID := c.Get("user_id").(int)
	settlement, err := h.procRepo.CreateSettlement(c.Request().Context(), req, userID)
	if err != nil {
		log.Printf("[procurement] create settlement error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create settlement")
	}
	return c.JSON(http.StatusCreated, settlement)
}

// ---- Admin: Create Vendors, Orders, Invoices ----

// POST /api/admin/vendors
func (h *ProcurementHandler) CreateVendor(c echo.Context) error {
	var req models.CreateVendorRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Name == "" || req.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name and code are required")
	}

	v := models.Vendor{
		Name:         req.Name,
		Code:         req.Code,
		ContactEmail: req.ContactEmail,
		ContactPhone: req.ContactPhone,
		Address:      req.Address,
	}
	created, err := h.procRepo.CreateVendor(c.Request().Context(), v)
	if err != nil {
		log.Printf("[procurement] create vendor error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create vendor")
	}
	return c.JSON(http.StatusCreated, created)
}

// POST /api/admin/orders
func (h *ProcurementHandler) CreateOrder(c echo.Context) error {
	var req models.CreateOrderRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.OrderNumber == "" || req.VendorID == 0 || req.TotalAmount <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "order_number, vendor_id, and total_amount are required")
	}

	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}
	userID := c.Get("user_id").(int)
	o := models.ProcurementOrder{
		OrderNumber: req.OrderNumber,
		VendorID:    req.VendorID,
		Department:  req.Department,
		CostCenter:  req.CostCenter,
		TotalAmount: req.TotalAmount,
		Currency:    currency,
		Description: req.Description,
	}
	created, err := h.procRepo.CreateOrder(c.Request().Context(), o, userID)
	if err != nil {
		log.Printf("[procurement] create order error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create order")
	}
	return c.JSON(http.StatusCreated, created)
}

// POST /api/admin/invoices
func (h *ProcurementHandler) CreateInvoice(c echo.Context) error {
	var req models.CreateInvoiceRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.InvoiceNumber == "" || req.VendorID == 0 || req.InvoiceAmount <= 0 || req.InvoiceDate == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "invoice_number, vendor_id, invoice_amount, and invoice_date are required")
	}

	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}
	inv := models.Invoice{
		InvoiceNumber: req.InvoiceNumber,
		VendorID:      req.VendorID,
		InvoiceAmount: req.InvoiceAmount,
		Currency:      currency,
		InvoiceDate:   req.InvoiceDate,
		DueDate:       req.DueDate,
		Notes:         req.Notes,
	}
	created, err := h.procRepo.CreateInvoice(c.Request().Context(), inv)
	if err != nil {
		log.Printf("[procurement] create invoice error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create invoice")
	}
	return c.JSON(http.StatusCreated, created)
}

// ---- Reviews ----

// POST /api/procurement/reviews
func (h *ProcurementHandler) CreateReview(c echo.Context) error {
	var req models.CreateReviewRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.VendorID == 0 || req.Rating < 1 || req.Rating > 5 || req.Body == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "vendor_id, rating (1-5), and body are required")
	}

	userID := c.Get("user_id").(int)
	review, err := h.procRepo.CreateReview(c.Request().Context(), req, userID)
	if err != nil {
		log.Printf("[procurement] create review error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create review")
	}
	return c.JSON(http.StatusCreated, review)
}

// POST /api/procurement/reviews/reply
func (h *ProcurementHandler) CreateMerchantReply(c echo.Context) error {
	var req models.CreateMerchantReplyRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.ReviewID == 0 || req.Body == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "review_id and body are required")
	}

	userID := c.Get("user_id").(int)
	// Get the review to find the vendor_id
	reviews, err := h.procRepo.GetReviews(c.Request().Context(), 0)
	if err != nil {
		log.Printf("[procurement] get reviews for reply error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to process reply")
	}
	var vendorID int
	for _, r := range reviews {
		if r.ID == req.ReviewID {
			vendorID = r.VendorID
			break
		}
	}
	if vendorID == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "review not found")
	}

	reply, err := h.procRepo.CreateMerchantReply(c.Request().Context(), req.ReviewID, vendorID, userID, req.Body)
	if err != nil {
		log.Printf("[procurement] create merchant reply error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create reply")
	}
	return c.JSON(http.StatusCreated, reply)
}

// ---- Disputes ----

// POST /api/procurement/disputes
func (h *ProcurementHandler) CreateDispute(c echo.Context) error {
	var req models.CreateDisputeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.ReviewID == 0 || req.VendorID == 0 || req.Reason == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "review_id, vendor_id, and reason are required")
	}

	userID := c.Get("user_id").(int)
	dispute, err := h.procRepo.CreateDispute(c.Request().Context(), req.ReviewID, req.VendorID, userID, req.Reason)
	if err != nil {
		log.Printf("[procurement] create dispute error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create dispute")
	}
	return c.JSON(http.StatusCreated, dispute)
}

// GET /api/procurement/reviews?vendor_id=1
func (h *ProcurementHandler) GetReviews(c echo.Context) error {
	vendorID := 0
	if v := c.QueryParam("vendor_id"); v != "" {
		vendorID, _ = strconv.Atoi(v)
	}
	reviews, err := h.procRepo.GetReviews(c.Request().Context(), vendorID)
	if err != nil {
		log.Printf("[procurement] get reviews error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve reviews")
	}

	// Apply personal data masking for non-admin users (hide reviewer_id)
	if !isAdmin(c) {
		return c.JSON(http.StatusOK, models.MaskReviews(reviews))
	}
	return c.JSON(http.StatusOK, reviews)
}

// ---- Disputes ----

// GET /api/procurement/disputes?status=created
func (h *ProcurementHandler) GetDisputes(c echo.Context) error {
	disputes, err := h.procRepo.GetDisputes(c.Request().Context(), c.QueryParam("status"))
	if err != nil {
		log.Printf("[procurement] get disputes error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve disputes")
	}

	if isAdmin(c) {
		// Admin/moderator: decrypt evidence metadata and return full dispute data
		for i := range disputes {
			decryptDisputeMetadata(&disputes[i])
		}
		return c.JSON(http.StatusOK, disputes)
	}

	// Non-admin: mask user-linked fields (created_by, arbitrated_by, evidence_metadata)
	return c.JSON(http.StatusOK, models.MaskDisputes(disputes))
}

// GET /api/procurement/disputes/:id
func (h *ProcurementHandler) GetDispute(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid dispute ID")
	}
	dispute, err := h.procRepo.GetDisputeByID(c.Request().Context(), id)
	if err != nil || dispute == nil {
		return echo.NewHTTPError(http.StatusNotFound, "dispute not found")
	}

	if isAdmin(c) {
		// Admin/moderator: decrypt evidence metadata and return full dispute data
		decryptDisputeMetadata(dispute)
		return c.JSON(http.StatusOK, dispute)
	}

	// Non-admin: mask user-linked fields
	masked := models.MaskDispute(*dispute)
	return c.JSON(http.StatusOK, masked)
}

// POST /api/procurement/disputes/transition
func (h *ProcurementHandler) TransitionDispute(c echo.Context) error {
	var req models.DisputeTransitionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	userID := c.Get("user_id").(int)
	if err := h.reconSvc.TransitionDispute(c.Request().Context(), req, userID); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "invalid transition") || strings.Contains(errMsg, "Invalid dispute transition") {
			log.Printf("[procurement] transition dispute conflict: %v", err)
			return echo.NewHTTPError(http.StatusConflict, "invalid state transition")
		}
		log.Printf("[procurement] transition dispute error: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "dispute transition failed")
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "dispute updated"})
}
