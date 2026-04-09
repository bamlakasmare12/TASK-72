package middleware

import (
	"log"
	"time"

	"github.com/labstack/echo/v4"
)

// RequestLogger logs each request with method, path, status, and duration.
// Sensitive headers (Authorization) are never logged.
func RequestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			duration := time.Since(start)

			status := c.Response().Status
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				}
			}

			log.Printf("[%s] %s %s - %d (%s) ip=%s",
				c.Request().Method,
				c.Request().URL.Path,
				c.Request().URL.RawQuery,
				status,
				duration,
				c.RealIP(),
			)

			return err
		}
	}
}
