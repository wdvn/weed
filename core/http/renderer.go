package http

import (
	"fmt"
	"io"
)

// Renderer is the interface that wraps the Render method.
// It mirrors echo's Renderer interface so existing renderers can be adapted.
type Renderer interface {
	// Render writes the rendered template to w.
	// name is the template name, data is the template data, c is the request context.
	Render(w io.Writer, name string, data interface{}, c *Ctx) error
}

// HTTPError represents an HTTP error with a status code and message.
// It mirrors echo.HTTPError so ocypode's error-handling middleware works as-is.
type HTTPError struct {
	Code    int
	Message interface{}
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTPError %d: %v", e.Code, e.Message)
}

// NewHTTPError creates a new HTTPError.
func NewHTTPError(code int, message ...interface{}) *HTTPError {
	he := &HTTPError{Code: code}
	if len(message) > 0 {
		he.Message = message[0]
	} else {
		he.Message = fmt.Sprintf("HTTP %d", code)
	}
	return he
}
