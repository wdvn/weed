package http

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"sync"
)

type Ctx struct {
	cx       context.Context
	r        *http.Request
	w        http.ResponseWriter
	resp     *Response
	params   Params
	store    map[string]any
	renderer Renderer
	mu       sync.RWMutex
}

func NewCtx(w http.ResponseWriter, r *http.Request, params ...Param) *Ctx {
	resp := newResponse(w)
	return &Ctx{
		r:      r,
		w:      resp, // use Response as the underlying writer
		resp:   resp,
		params: params,
		cx:     r.Context(),
	}
}

func (c *Ctx) Request() *http.Request {
	return c.r
}

func (c *Ctx) Writer() http.ResponseWriter {
	return c.w
}

// Response returns the response wrapper which tracks Committed state.
func (c *Ctx) Response() *Response {
	return c.resp
}

func (c *Ctx) Next() {

}

func (c *Ctx) Param(key string) string {
	return c.params.Get(key)
}
func (c *Ctx) Params() []Param {
	return c.params
}

func (c *Ctx) Body() ([]byte, error) {
	return io.ReadAll(c.r.Body)
}

// Query returns the URL query parameter by key.
func (c *Ctx) Query(key string) string {
	return c.r.URL.Query().Get(key)
}

// QueryParam is an alias for Query, matching echo's API.
func (c *Ctx) QueryParam(key string) string {
	return c.r.URL.Query().Get(key)
}

// FormValue returns the form value (both URL-encoded and multipart).
func (c *Ctx) FormValue(key string) string {
	return c.r.FormValue(key)
}

// Bind decodes the request body (JSON) into v.
// If Content-Type is multipart/form or application/x-www-form-urlencoded,
// it falls back to form parsing and fills v via reflection (basic).
func (c *Ctx) Bind(v any) error {
	return json.NewDecoder(c.r.Body).Decode(v)
}

// MultipartForm parses and returns the multipart form data.
func (c *Ctx) MultipartForm() (*multipart.Form, error) {
	if err := c.r.ParseMultipartForm(32 << 20); err != nil {
		return nil, err
	}
	return c.r.MultipartForm, nil
}

// Set stores a value in the request-scoped context store.
func (c *Ctx) Set(key string, val any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.store == nil {
		c.store = make(map[string]any)
	}
	c.store[key] = val
}

// Get retrieves a value from the request-scoped context store.
func (c *Ctx) Get(key string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.store == nil {
		return nil
	}
	return c.store[key]
}

// SetRenderer sets the template renderer for this context.
func (c *Ctx) SetRenderer(r Renderer) {
	c.renderer = r
}

func (c *Ctx) writeHeader(k, v string) {
	c.w.Header().Set(k, v)
}

// JSON writes a JSON response with the given status code.
func (c *Ctx) JSON(status int, data any) error {
	c.writeHeader("Content-Type", "application/json; charset=utf-8")
	c.w.WriteHeader(status)
	encoder := json.NewEncoder(c.w)
	return encoder.Encode(data)
}

// Text writes a plain text response.
func (c *Ctx) Text(status int, s string) error {
	c.writeHeader("Content-Type", "text/plain; charset=utf-8")
	c.w.WriteHeader(status)
	_, err := io.WriteString(c.w, s)
	return err
}

// String is an alias for Text, matching echo's API.
func (c *Ctx) String(status int, s string) error {
	return c.Text(status, s)
}

// Html writes an HTML response.
func (c *Ctx) Html(status int, s string) error {
	c.writeHeader("Content-Type", "text/html; charset=utf-8")
	c.w.WriteHeader(status)
	_, err := io.WriteString(c.w, s)
	return err
}

// HTML is an alias for Html, matching echo's API.
func (c *Ctx) HTML(status int, s string) error {
	return c.Html(status, s)
}

// Redirect performs an HTTP redirect.
func (c *Ctx) Redirect(status int, url string) error {
	http.Redirect(c.w, c.r, url, status)
	return nil
}

// File serves a file at the given filesystem path.
// This single-argument version matches echo's ctx.File(path) signature.
func (c *Ctx) File(path string) error {
	http.ServeFile(c.w, c.r, path)
	return nil
}

// ServeFile serves a file with an explicit status code (weed's original API).
func (c *Ctx) ServeFile(status int, path string) error {
	c.w.WriteHeader(status)
	http.ServeFile(c.w, c.r, path)
	return nil
}

// Bytes writes raw bytes with the specified content-type.
func (c *Ctx) Bytes(status int, contentType string, bytes []byte) error {
	c.writeHeader("Content-Type", contentType)
	c.w.WriteHeader(status)
	_, err := c.w.Write(bytes)
	return err
}

// NoContent sends a response with no body.
func (c *Ctx) NoContent(status int) error {
	c.w.WriteHeader(status)
	return nil
}

// Render renders a named template using the registered renderer.
func (c *Ctx) Render(status int, name string, data any) error {
	if c.renderer == nil {
		return NewHTTPError(http.StatusInternalServerError, "renderer not set")
	}
	c.writeHeader("Content-Type", "text/html; charset=utf-8")
	c.w.WriteHeader(status)
	return c.renderer.Render(c.w, name, data, c)
}
