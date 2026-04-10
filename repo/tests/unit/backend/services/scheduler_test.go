package services_test

import (
	"testing"
	"time"

	"wlpr-portal/internal/services"
)

func TestParseCronInterval_EveryNMinutes(t *testing.T) {
	tests := []struct {
		expr     string
		expected time.Duration
	}{
		{"*/5 * * * *", 5 * time.Minute},
		{"*/15 * * * *", 15 * time.Minute},
		{"*/1 * * * *", 1 * time.Minute},
		{"*/60 * * * *", 60 * time.Minute},
	}

	for _, tt := range tests {
		result := services.ParseCronInterval(tt.expr)
		if result != tt.expected {
			t.Errorf("services.ParseCronInterval(%q) = %v, want %v", tt.expr, result, tt.expected)
		}
	}
}

func TestParseCronInterval_DailyJob(t *testing.T) {
	result := services.ParseCronInterval("0 2 * * *")
	if result != 24*time.Hour {
		t.Errorf("daily cron should parse to 24h, got %v", result)
	}
}

func TestParseCronInterval_WeeklyJob(t *testing.T) {
	result := services.ParseCronInterval("0 3 * * 1")
	if result != 7*24*time.Hour {
		t.Errorf("weekly cron should parse to 168h, got %v", result)
	}
}

func TestParseCronInterval_InvalidExpr(t *testing.T) {
	tests := []string{
		"",
		"invalid",
		"* *",
	}

	for _, expr := range tests {
		result := services.ParseCronInterval(expr)
		if result != 0 {
			t.Errorf("services.ParseCronInterval(%q) = %v, want 0", expr, result)
		}
	}
}

func TestParseCronInterval_InvalidMinuteValue(t *testing.T) {
	result := services.ParseCronInterval("*/abc * * * *")
	if result != 0 {
		t.Errorf("invalid minute value should return 0, got %v", result)
	}
}

func TestParseCronInterval_ZeroMinutes(t *testing.T) {
	result := services.ParseCronInterval("*/0 * * * *")
	if result != 0 {
		t.Errorf("*/0 should return 0 (invalid), got %v", result)
	}
}
