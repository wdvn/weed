package http

import "net/http"

// Response is a thin wrapper around http.ResponseWriter that tracks
// whether the response header has already been written (Committed).
// This mirrors the echo.Response type used by ocypode's middleware.
type Response struct {
	Writer    http.ResponseWriter
	Committed bool
	Status    int
}

func newResponse(w http.ResponseWriter) *Response {
	return &Response{Writer: w}
}

func (r *Response) Header() http.Header {
	return r.Writer.Header()
}

func (r *Response) Write(b []byte) (int, error) {
	if !r.Committed {
		r.WriteHeader(http.StatusOK)
	}
	return r.Writer.Write(b)
}

func (r *Response) WriteHeader(code int) {
	if r.Committed {
		return
	}
	r.Status = code
	r.Writer.WriteHeader(code)
	r.Committed = true
}
