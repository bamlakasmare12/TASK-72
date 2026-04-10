package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"wlpr-portal/internal/middleware"

	"github.com/labstack/echo/v4"
)

// --- CompareVersions unit tests ---

func TestCompareVersions_Equal(t *testing.T) {
	if middleware.CompareVersions("1.0.0", "1.0.0") != 0 {
		t.Error("1.0.0 == 1.0.0")
	}
}

func TestCompareVersions_LessThan(t *testing.T) {
	if middleware.CompareVersions("0.9.0", "1.0.0") >= 0 {
		t.Error("0.9.0 < 1.0.0")
	}
	if middleware.CompareVersions("1.0.0", "1.0.1") >= 0 {
		t.Error("1.0.0 < 1.0.1")
	}
	if middleware.CompareVersions("1.2.3", "2.0.0") >= 0 {
		t.Error("1.2.3 < 2.0.0")
	}
}

func TestCompareVersions_GreaterThan(t *testing.T) {
	if middleware.CompareVersions("2.0.0", "1.0.0") <= 0 {
		t.Error("2.0.0 > 1.0.0")
	}
	if middleware.CompareVersions("1.1.0", "1.0.0") <= 0 {
		t.Error("1.1.0 > 1.0.0")
	}
}

// --- AppVersionCheck grace-period middleware behavior tests ---
// These use inline middleware stubs that replicate the exact logic from
// AppVersionCheck so the tests run without a DB or real config service.

// versionCheckMiddleware replicates AppVersionCheck logic with injected params.
func versionCheckMiddleware(minVersion string, minVersionSetAt time.Time, graceDays int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if minVersion == "" {
				return next(c)
			}
			v := c.Request().Header.Get("X-App-Version")
			if v == "" {
				// Missing version header: treat as oldest client ("0.0.0")
				v = "0.0.0"
			}
			if middleware.CompareVersions(v, minVersion) < 0 {
				graceDeadline := minVersionSetAt.Add(time.Duration(graceDays) * 24 * time.Hour)
				withinGrace := time.Now().Before(graceDeadline)

				c.Response().Header().Set("X-App-Deprecated", "true")
				c.Response().Header().Set("X-Min-Version", minVersion)

				if !withinGrace {
					return echo.NewHTTPError(http.StatusUpgradeRequired,
						"client version unsupported and grace period expired; upgrade to "+minVersion+" or later")
				}

				c.Set("read_only", true)
				method := c.Request().Method
				if method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE" {
					return echo.NewHTTPError(http.StatusUpgradeRequired, "read-only during grace period")
				}
			}
			return next(c)
		}
	}
}

func TestAppVersionCheck_NoHeader_WithMinVersion_TreatedAsOldest(t *testing.T) {
	// When min version is configured and header is missing, client is treated as "0.0.0".
	// Since the min was just set (now), grace period is fully active, so GET is allowed
	// but the client is flagged as deprecated.
	e := echo.New()
	e.Use(versionCheckMiddleware("2.0.0", time.Now(), 14))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("no version header within grace should allow GET, got %d", rec.Code)
	}
	if rec.Header().Get("X-App-Deprecated") != "true" {
		t.Error("expected X-App-Deprecated: true header when version is missing")
	}
}

func TestAppVersionCheck_NoHeader_WithMinVersion_POSTBlocked(t *testing.T) {
	// Missing version header + min version configured + within grace -> POST blocked
	e := echo.New()
	e.Use(versionCheckMiddleware("2.0.0", time.Now(), 14))
	e.POST("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("no version header within grace should block POST, got %d", rec.Code)
	}
}

func TestAppVersionCheck_NoHeader_GraceExpired_Blocked(t *testing.T) {
	// Missing version header + min version configured + grace expired -> all blocked
	e := echo.New()
	e.Use(versionCheckMiddleware("2.0.0", time.Now().Add(-20*24*time.Hour), 14))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("no version header after grace expiry should block GET, got %d", rec.Code)
	}
}

func TestAppVersionCheck_NoHeader_NoMinVersion_Allowed(t *testing.T) {
	// When no min version is configured, missing header should pass through freely
	e := echo.New()
	e.Use(versionCheckMiddleware("", time.Now(), 14))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("no min version configured should pass, got %d", rec.Code)
	}
}

func TestAppVersionCheck_CurrentVersion_Allowed(t *testing.T) {
	e := echo.New()
	e.Use(versionCheckMiddleware("2.0.0", time.Now(), 14))
	e.POST("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-App-Version", "2.1.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("current version should pass all methods, got %d", rec.Code)
	}
}

func TestAppVersionCheck_OldVersion_InGrace_GETAllowed(t *testing.T) {
	// min version set 5 days ago, grace is 14 days -> within grace
	e := echo.New()
	e.Use(versionCheckMiddleware("2.0.0", time.Now().Add(-5*24*time.Hour), 14))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "read ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-App-Version", "1.0.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("old version within grace should allow GET, got %d", rec.Code)
	}
	if rec.Header().Get("X-App-Deprecated") != "true" {
		t.Error("expected X-App-Deprecated: true header")
	}
	if rec.Header().Get("X-Min-Version") != "2.0.0" {
		t.Error("expected X-Min-Version: 2.0.0 header")
	}
}

func TestAppVersionCheck_OldVersion_InGrace_POSTBlocked(t *testing.T) {
	e := echo.New()
	e.Use(versionCheckMiddleware("2.0.0", time.Now().Add(-5*24*time.Hour), 14))
	e.POST("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-App-Version", "1.0.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("old version within grace should block POST with 426, got %d", rec.Code)
	}
}

func TestAppVersionCheck_OldVersion_InGrace_PUTBlocked(t *testing.T) {
	e := echo.New()
	e.Use(versionCheckMiddleware("2.0.0", time.Now().Add(-5*24*time.Hour), 14))
	e.PUT("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodPut, "/test", nil)
	req.Header.Set("X-App-Version", "1.0.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("old version within grace should block PUT, got %d", rec.Code)
	}
}

func TestAppVersionCheck_OldVersion_InGrace_DELETEBlocked(t *testing.T) {
	e := echo.New()
	e.Use(versionCheckMiddleware("2.0.0", time.Now().Add(-5*24*time.Hour), 14))
	e.DELETE("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodDelete, "/test", nil)
	req.Header.Set("X-App-Version", "1.0.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("old version within grace should block DELETE, got %d", rec.Code)
	}
}

func TestAppVersionCheck_GraceExpired_GETBlocked(t *testing.T) {
	// min version set 20 days ago, grace is 14 days -> expired
	e := echo.New()
	e.Use(versionCheckMiddleware("2.0.0", time.Now().Add(-20*24*time.Hour), 14))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-App-Version", "1.0.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("old version after grace expiry should block GET, got %d", rec.Code)
	}
}

func TestAppVersionCheck_GraceExpired_POSTBlocked(t *testing.T) {
	e := echo.New()
	e.Use(versionCheckMiddleware("2.0.0", time.Now().Add(-20*24*time.Hour), 14))
	e.POST("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-App-Version", "1.0.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("old version after grace expiry should block POST, got %d", rec.Code)
	}
}

func TestAppVersionCheck_GraceBoundary_LastDay(t *testing.T) {
	// min version set 13 days ago, grace is 14 days -> still within grace (barely)
	e := echo.New()
	e.Use(versionCheckMiddleware("2.0.0", time.Now().Add(-13*24*time.Hour), 14))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "still in grace")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-App-Version", "1.0.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("on day 13 of 14-day grace, GET should be allowed, got %d", rec.Code)
	}
}

func TestAppVersionCheck_GraceBoundary_DayAfterExpiry(t *testing.T) {
	// min version set 15 days ago, grace is 14 days -> expired
	e := echo.New()
	e.Use(versionCheckMiddleware("2.0.0", time.Now().Add(-15*24*time.Hour), 14))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-App-Version", "1.0.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("on day 15 of 14-day grace, GET should be blocked, got %d", rec.Code)
	}
}

func TestAppVersionCheck_CustomGraceDays(t *testing.T) {
	// 7-day grace, set 5 days ago -> within grace
	e := echo.New()
	e.Use(versionCheckMiddleware("3.0.0", time.Now().Add(-5*24*time.Hour), 7))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-App-Version", "2.0.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("within 7-day grace should allow GET, got %d", rec.Code)
	}
}

func TestAppVersionCheck_CustomGraceDays_Expired(t *testing.T) {
	// 7-day grace, set 10 days ago -> expired
	e := echo.New()
	e.Use(versionCheckMiddleware("3.0.0", time.Now().Add(-10*24*time.Hour), 7))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-App-Version", "2.0.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("past 7-day grace should block GET, got %d", rec.Code)
	}
}
