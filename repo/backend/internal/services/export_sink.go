package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"wlpr-portal/internal/models"
)

// ExportSinkService handles offline finance export to file-drop directories
// and LAN-based webhooks with retry and compensation logic.
type ExportSinkService struct {
	configSvc *ConfigService
}

func NewExportSinkService(configSvc *ConfigService) *ExportSinkService {
	return &ExportSinkService{configSvc: configSvc}
}

// ExportConfig holds resolved export sink configuration.
type ExportConfig struct {
	FileDropDir    string
	WebhookURL     string
	MaxRetries     int
	RetryDelaySecs int
}

// GetExportConfig resolves export sink configuration from the config service.
func (s *ExportSinkService) GetExportConfig() ExportConfig {
	cfg := ExportConfig{
		MaxRetries:     3,
		RetryDelaySecs: 5,
	}

	if dir, ok := s.configSvc.GetConfig("export.file_drop_dir"); ok && dir != "" {
		cfg.FileDropDir = dir
	}
	if url, ok := s.configSvc.GetConfig("export.webhook_url"); ok && url != "" {
		cfg.WebhookURL = url
	}
	if v, ok := s.configSvc.GetConfig("export.max_retries"); ok {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.MaxRetries = n
		}
	}
	if v, ok := s.configSvc.GetConfig("export.retry_delay_seconds"); ok {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.RetryDelaySecs = n
		}
	}
	return cfg
}

// ExportLedgerToSinks exports ledger entries to configured sinks (file-drop and/or webhook).
// Returns nil if no sinks are configured (browser CSV download is still available).
func (s *ExportSinkService) ExportLedgerToSinks(ctx context.Context, entries []models.LedgerEntry) error {
	cfg := s.GetExportConfig()

	if cfg.FileDropDir == "" && cfg.WebhookURL == "" {
		return nil // No sinks configured; browser download only
	}

	csvData, err := LedgerToCSV(entries)
	if err != nil {
		return fmt.Errorf("failed to generate CSV: %w", err)
	}

	var errs []error

	if cfg.FileDropDir != "" {
		if err := s.writeFileDrop(cfg.FileDropDir, "ledger_export", csvData); err != nil {
			log.Printf("[export-sink] file-drop error: %v", err)
			errs = append(errs, fmt.Errorf("file-drop: %w", err))
		} else {
			log.Printf("[export-sink] ledger exported to file-drop: %s", cfg.FileDropDir)
		}
	}

	if cfg.WebhookURL != "" {
		if err := s.sendWebhook(ctx, cfg, "ledger_export", csvData); err != nil {
			log.Printf("[export-sink] webhook error: %v", err)
			errs = append(errs, fmt.Errorf("webhook: %w", err))
		} else {
			log.Printf("[export-sink] ledger exported via webhook: %s", cfg.WebhookURL)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("export sink errors: %v", errs)
	}
	return nil
}

// ExportSettlementsToSinks exports settlements to configured sinks.
func (s *ExportSinkService) ExportSettlementsToSinks(ctx context.Context, settlements []models.Settlement) error {
	cfg := s.GetExportConfig()

	if cfg.FileDropDir == "" && cfg.WebhookURL == "" {
		return nil
	}

	csvData, err := SettlementsToCSV(settlements)
	if err != nil {
		return fmt.Errorf("failed to generate CSV: %w", err)
	}

	var errs []error

	if cfg.FileDropDir != "" {
		if err := s.writeFileDrop(cfg.FileDropDir, "settlements_export", csvData); err != nil {
			log.Printf("[export-sink] file-drop error: %v", err)
			errs = append(errs, fmt.Errorf("file-drop: %w", err))
		} else {
			log.Printf("[export-sink] settlements exported to file-drop: %s", cfg.FileDropDir)
		}
	}

	if cfg.WebhookURL != "" {
		if err := s.sendWebhook(ctx, cfg, "settlements_export", csvData); err != nil {
			log.Printf("[export-sink] webhook error: %v", err)
			errs = append(errs, fmt.Errorf("webhook: %w", err))
		} else {
			log.Printf("[export-sink] settlements exported via webhook: %s", cfg.WebhookURL)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("export sink errors: %v", errs)
	}
	return nil
}

// writeFileDrop writes a CSV file to the configured file-drop directory.
func (s *ExportSinkService) writeFileDrop(dir, prefix string, data []byte) error {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create export directory %s: %w", dir, err)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.csv", prefix, timestamp)
	filePath := filepath.Join(dir, filename)

	if err := os.WriteFile(filePath, data, 0640); err != nil {
		return fmt.Errorf("failed to write export file %s: %w", filePath, err)
	}

	log.Printf("[export-sink] file written: %s (%d bytes)", filePath, len(data))
	return nil
}

// sendWebhook sends export data to a LAN-based webhook with retry/compensation.
func (s *ExportSinkService) sendWebhook(ctx context.Context, cfg ExportConfig, exportType string, data []byte) error {
	payload := map[string]interface{}{
		"export_type": exportType,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"data_base64": data,
		"format":      "csv",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return fmt.Errorf("context cancelled during webhook retry: %w", ctx.Err())
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.WebhookURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create webhook request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Export-Type", exportType)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d/%d: %w", attempt, cfg.MaxRetries, err)
			log.Printf("[export-sink] webhook attempt %d/%d failed: %v", attempt, cfg.MaxRetries, err)
			if attempt < cfg.MaxRetries {
				time.Sleep(time.Duration(cfg.RetryDelaySecs) * time.Second)
			}
			continue
		}

		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("[export-sink] webhook succeeded on attempt %d/%d", attempt, cfg.MaxRetries)
			return nil
		}

		lastErr = fmt.Errorf("attempt %d/%d: HTTP %d", attempt, cfg.MaxRetries, resp.StatusCode)
		log.Printf("[export-sink] webhook attempt %d/%d returned HTTP %d", attempt, cfg.MaxRetries, resp.StatusCode)
		if attempt < cfg.MaxRetries {
			time.Sleep(time.Duration(cfg.RetryDelaySecs) * time.Second)
		}
	}

	// Compensation: write to file-drop as fallback if webhook fails
	fileFallbackDir, _ := s.configSvc.GetConfig("export.file_drop_dir")
	if fileFallbackDir != "" {
		log.Printf("[export-sink] webhook failed after %d retries; compensating with file-drop fallback", cfg.MaxRetries)
		if err := s.writeFileDrop(fileFallbackDir, exportType+"_webhook_fallback", data); err != nil {
			log.Printf("[export-sink] fallback file-drop also failed: %v", err)
		}
	}

	return fmt.Errorf("webhook failed after %d retries: %w", cfg.MaxRetries, lastErr)
}

// LedgerToCSV converts ledger entries to CSV bytes.
func LedgerToCSV(entries []models.LedgerEntry) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

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
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SettlementsToCSV converts settlements to CSV bytes.
func SettlementsToCSV(settlements []models.Settlement) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

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
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
