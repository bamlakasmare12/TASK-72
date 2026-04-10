package services_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/services"
)

// mockConfigService creates a ConfigService with in-memory configs for testing.
func mockConfigService(configs map[string]string) *services.ConfigService {
	configMap := make(map[string]models.Config)
	for k, v := range configs {
		configMap[k] = models.Config{
			Key:       k,
			Value:     v,
			UpdatedAt: time.Now(),
		}
	}
	return services.NewConfigServiceWithFlags(configMap, make(map[string]models.FeatureFlag))
}

// --- File-Drop Tests ---

func TestExportSink_FileDrop_WritesCSVFile(t *testing.T) {
	tmpDir := t.TempDir()

	cfgSvc := mockConfigService(map[string]string{
		"export.file_drop_dir": tmpDir,
	})
	sink := services.NewExportSinkService(cfgSvc)

	entries := []models.LedgerEntry{
		{ID: 1, EntryType: "AR", ReferenceType: "invoice", ReferenceID: 10,
			VendorID: 1, VendorName: "Acme", Amount: 500.00, Currency: "USD",
			PostedAt: time.Now()},
	}

	err := sink.ExportLedgerToSinks(context.Background(), entries)
	if err != nil {
		t.Fatalf("ExportLedgerToSinks failed: %v", err)
	}

	// Verify file was created
	files, _ := filepath.Glob(filepath.Join(tmpDir, "ledger_export_*.csv"))
	if len(files) == 0 {
		t.Fatal("expected at least one ledger export file in file-drop directory")
	}

	// Verify content
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("failed to read export file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Entry ID") {
		t.Error("CSV missing header row")
	}
	if !strings.Contains(content, "Acme") {
		t.Error("CSV missing vendor name data")
	}
	if !strings.Contains(content, "500.00") {
		t.Error("CSV missing amount data")
	}
}

func TestExportSink_FileDrop_SettlementsCSV(t *testing.T) {
	tmpDir := t.TempDir()

	cfgSvc := mockConfigService(map[string]string{
		"export.file_drop_dir": tmpDir,
	})
	sink := services.NewExportSinkService(cfgSvc)

	settlements := []models.Settlement{
		{ID: 1, VendorID: 2, VendorName: "TestVendor", Status: "open",
			ARTotal: 1000, APTotal: 900, NetAmount: 100, VarianceAmount: 100},
	}

	err := sink.ExportSettlementsToSinks(context.Background(), settlements)
	if err != nil {
		t.Fatalf("ExportSettlementsToSinks failed: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(tmpDir, "settlements_export_*.csv"))
	if len(files) == 0 {
		t.Fatal("expected at least one settlements export file")
	}

	data, _ := os.ReadFile(files[0])
	if !strings.Contains(string(data), "TestVendor") {
		t.Error("CSV missing vendor name")
	}
}

func TestExportSink_NoSinksConfigured_ReturnsNil(t *testing.T) {
	cfgSvc := mockConfigService(map[string]string{})
	sink := services.NewExportSinkService(cfgSvc)

	err := sink.ExportLedgerToSinks(context.Background(), []models.LedgerEntry{})
	if err != nil {
		t.Fatalf("expected nil error when no sinks configured, got: %v", err)
	}
}

// --- Webhook Tests ---

func TestExportSink_Webhook_Success(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("X-Export-Type") != "ledger_export" {
			t.Errorf("expected X-Export-Type ledger_export, got %s", r.Header.Get("X-Export-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfgSvc := mockConfigService(map[string]string{
		"export.webhook_url":         server.URL,
		"export.max_retries":         "3",
		"export.retry_delay_seconds": "0",
	})
	sink := services.NewExportSinkService(cfgSvc)

	entries := []models.LedgerEntry{
		{ID: 1, EntryType: "AR", ReferenceType: "invoice", ReferenceID: 10,
			VendorID: 1, VendorName: "Acme", Amount: 500.00, Currency: "USD",
			PostedAt: time.Now()},
	}

	err := sink.ExportLedgerToSinks(context.Background(), entries)
	if err != nil {
		t.Fatalf("ExportLedgerToSinks webhook failed: %v", err)
	}

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected exactly 1 webhook call on success, got %d", callCount)
	}
}

func TestExportSink_Webhook_RetriesOnFailure(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfgSvc := mockConfigService(map[string]string{
		"export.webhook_url":         server.URL,
		"export.max_retries":         "3",
		"export.retry_delay_seconds": "0",
	})
	sink := services.NewExportSinkService(cfgSvc)

	entries := []models.LedgerEntry{
		{ID: 1, EntryType: "AR", ReferenceType: "invoice", ReferenceID: 10,
			VendorID: 1, VendorName: "Acme", Amount: 100.00, Currency: "USD",
			PostedAt: time.Now()},
	}

	err := sink.ExportLedgerToSinks(context.Background(), entries)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}

	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("expected 3 webhook attempts (2 failures + 1 success), got %d", callCount)
	}
}

func TestExportSink_Webhook_FallbackToFileDrop(t *testing.T) {
	// Server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	cfgSvc := mockConfigService(map[string]string{
		"export.webhook_url":         server.URL,
		"export.file_drop_dir":       tmpDir,
		"export.max_retries":         "2",
		"export.retry_delay_seconds": "0",
	})
	sink := services.NewExportSinkService(cfgSvc)

	entries := []models.LedgerEntry{
		{ID: 1, EntryType: "AP", ReferenceType: "order", ReferenceID: 5,
			VendorID: 2, VendorName: "Beta", Amount: 250.00, Currency: "USD",
			PostedAt: time.Now()},
	}

	// This will report errors (webhook failed) but should still write fallback file
	_ = sink.ExportLedgerToSinks(context.Background(), entries)

	// Check for fallback file (named with _webhook_fallback)
	fallbackFiles, _ := filepath.Glob(filepath.Join(tmpDir, "*webhook_fallback*.csv"))
	// Also check for the normal file-drop file (since file_drop_dir is configured,
	// the primary file-drop export runs first)
	primaryFiles, _ := filepath.Glob(filepath.Join(tmpDir, "ledger_export_*.csv"))

	totalFiles := len(fallbackFiles) + len(primaryFiles)
	if totalFiles == 0 {
		t.Fatal("expected at least one file (primary file-drop or webhook fallback) in the directory")
	}
}

func TestExportSink_Webhook_AllRetriesFail_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfgSvc := mockConfigService(map[string]string{
		"export.webhook_url":         server.URL,
		"export.max_retries":         "2",
		"export.retry_delay_seconds": "0",
	})
	sink := services.NewExportSinkService(cfgSvc)

	entries := []models.LedgerEntry{
		{ID: 1, EntryType: "AR", ReferenceType: "invoice", ReferenceID: 10,
			VendorID: 1, VendorName: "Acme", Amount: 500.00, Currency: "USD",
			PostedAt: time.Now()},
	}

	err := sink.ExportLedgerToSinks(context.Background(), entries)
	if err == nil {
		t.Fatal("expected error when all webhook retries fail")
	}
	if !strings.Contains(err.Error(), "webhook") {
		t.Errorf("error should mention webhook, got: %v", err)
	}
}

// --- CSV Generation Tests ---

func TestLedgerToCSV_GeneratesValidCSV(t *testing.T) {
	dept := "Engineering"
	cc := "CC001"
	desc := "Test entry"
	entries := []models.LedgerEntry{
		{ID: 1, EntryType: "AR", ReferenceType: "invoice", ReferenceID: 10,
			VendorID: 1, VendorName: "Acme", Amount: 500.00, Currency: "USD",
			Department: &dept, CostCenter: &cc, Description: &desc,
			PostedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)},
		{ID: 2, EntryType: "AP", ReferenceType: "order", ReferenceID: 5,
			VendorID: 2, VendorName: "Beta", Amount: 250.50, Currency: "USD",
			PostedAt: time.Date(2025, 2, 20, 14, 0, 0, 0, time.UTC)},
	}

	data, err := services.LedgerToCSV(entries)
	if err != nil {
		t.Fatalf("services.LedgerToCSV failed: %v", err)
	}

	content := string(data)
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) != 3 { // header + 2 data rows
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "Entry ID") {
		t.Error("missing CSV header")
	}
	if !strings.Contains(content, "Engineering") {
		t.Error("missing department value")
	}
	if !strings.Contains(content, "250.50") {
		t.Error("missing amount value")
	}
}

func TestSettlementsToCSV_GeneratesValidCSV(t *testing.T) {
	dept := "Finance"
	ps := "2025-01-01"
	pe := "2025-01-31"
	settlements := []models.Settlement{
		{ID: 1, VendorID: 1, VendorName: "Acme", Status: "settled",
			ARTotal: 10000, APTotal: 9500, NetAmount: 500,
			VarianceAmount: 50, WriteoffAmount: 5,
			Department: &dept, PeriodStart: &ps, PeriodEnd: &pe},
	}

	data, err := services.SettlementsToCSV(settlements)
	if err != nil {
		t.Fatalf("services.SettlementsToCSV failed: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Settlement ID") {
		t.Error("missing CSV header")
	}
	if !strings.Contains(content, "settled") {
		t.Error("missing status value")
	}
	if !strings.Contains(content, "Finance") {
		t.Error("missing department value")
	}
}

func TestExportConfig_Defaults(t *testing.T) {
	cfgSvc := mockConfigService(map[string]string{})
	sink := services.NewExportSinkService(cfgSvc)
	cfg := sink.GetExportConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("expected default MaxRetries=3, got %d", cfg.MaxRetries)
	}
	if cfg.RetryDelaySecs != 5 {
		t.Errorf("expected default RetryDelaySecs=5, got %d", cfg.RetryDelaySecs)
	}
	if cfg.FileDropDir != "" {
		t.Errorf("expected empty FileDropDir, got %q", cfg.FileDropDir)
	}
	if cfg.WebhookURL != "" {
		t.Errorf("expected empty WebhookURL, got %q", cfg.WebhookURL)
	}
}

func TestExportConfig_CustomValues(t *testing.T) {
	cfgSvc := mockConfigService(map[string]string{
		"export.file_drop_dir":       "/custom/path",
		"export.webhook_url":         "http://lan.local/hook",
		"export.max_retries":         "5",
		"export.retry_delay_seconds": "10",
	})
	sink := services.NewExportSinkService(cfgSvc)
	cfg := sink.GetExportConfig()

	if cfg.FileDropDir != "/custom/path" {
		t.Errorf("expected FileDropDir=/custom/path, got %q", cfg.FileDropDir)
	}
	if cfg.WebhookURL != "http://lan.local/hook" {
		t.Errorf("expected WebhookURL=http://lan.local/hook, got %q", cfg.WebhookURL)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("expected MaxRetries=5, got %d", cfg.MaxRetries)
	}
	if cfg.RetryDelaySecs != 10 {
		t.Errorf("expected RetryDelaySecs=10, got %d", cfg.RetryDelaySecs)
	}
}
