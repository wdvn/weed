// Package middleware provides built-in middlewares for weed,
// offering a drop-in replacement for the echo middleware package
// functions used in ocypode.
package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"

	whttp "github.com/wdvn/weed/core/http"
)

// Logger returns a middleware that logs each request's method, path,
// status and elapsed time — equivalent to echo/middleware.Logger().
func Logger() whttp.MiddlewareFunc {
	return func(next whttp.HandlerFunc) whttp.HandlerFunc {
		return func(c *whttp.Ctx) error {
			start := time.Now()
			err := next(c)
			resp := c.Response()
			status := 200
			if resp != nil {
				status = resp.Status
				if status == 0 && resp.Committed {
					status = 200
				}
			}
			log.Printf("[WEED] %s %s | %d | %v",
				c.Request().Method,
				c.Request().URL.Path,
				status,
				time.Since(start),
			)
			return err
		}
	}
}

// Recover returns a middleware that recovers from panics and returns
// HTTP 500 — equivalent to echo/middleware.Recover().
func Recover() whttp.MiddlewareFunc {
	return func(next whttp.HandlerFunc) whttp.HandlerFunc {
		return func(c *whttp.Ctx) (err error) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[WEED] PANIC recovered: %v\n%s", r, debug.Stack())
					_ = c.Text(http.StatusInternalServerError, "Internal Server Error")
				}
			}()
			return next(c)
		}
	}
}

// Secure returns a middleware that sets common security headers
// — equivalent to echo/middleware.Secure().
func Secure() whttp.MiddlewareFunc {
	return func(next whttp.HandlerFunc) whttp.HandlerFunc {
		return func(c *whttp.Ctx) error {
			c.Writer().Header().Set("X-Content-Type-Options", "nosniff")
			c.Writer().Header().Set("X-Frame-Options", "SAMEORIGIN")
			c.Writer().Header().Set("X-XSS-Protection", "1; mode=block")
			c.Writer().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			return next(c)
		}
	}
}

// csrfTokenLen is the byte length of the CSRF token.
const csrfTokenLen = 32

// CSRF returns a lightweight CSRF middleware that:
//  1. On safe methods (GET, HEAD, OPTIONS, TRACE) — generates a token and sets a cookie.
//  2. On unsafe methods — validates the X-CSRF-Token header or _csrf form field against the cookie.
//
// It mirrors the essential behaviour of echo/middleware.CSRF() for ocypode's use-case.
//func CSRF() whttp.MiddlewareFunc {
//	return func(next whttp.HandlerFunc) whttp.HandlerFunc {
//		return func(c *whttp.Ctx) error {
//			req := c.Request()
//			method := req.Method
//
//			// Safe methods — skip validation, just ensure the cookie exists
//			safe := method == http.MethodGet || method == http.MethodHead ||
//				method == http.MethodOptions || method == http.MethodTrace
//			if safe {
//				return next(c)
//			}
//
//			// Unsafe methods — validate token
//			cookie, err := req.Cookie("_csrf")
//			if err != nil || cookie.Value == "" {
//				return fmt.Errorf("missing CSRF token")
//			}
//			expected := cookie.Value
//
//			// Check X-CSRF-Token header first, then _csrf form field
//			token := req.Header.Get("X-CSRF-Token")
//			if token == "" {
//				token = req.FormValue("_csrf")
//			}
//
//			if token != expected {
//				return fmt.Errorf("invalid CSRF token")
//			}
//			return next(c)
//		}
//	}
//}
