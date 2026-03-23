package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

type Ctx struct {
	cx     context.Context
	r      *http.Request
	w      http.ResponseWriter
	params Params
}

func NewCtx(w http.ResponseWriter, r *http.Request, params ...Param) *Ctx {
	return &Ctx{
		r:      r,
		w:      w,
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

func (c *Ctx) Query(key string) string {
	return c.r.URL.Query().Get(key)
}

func (c *Ctx) writeHeader(k, v string) {
	c.w.Header().Set(k, v)
}

func (c *Ctx) JSON(status int, data any) error {
	c.writeHeader("Content-Type", "application/json; charset=utf-8")
	c.w.WriteHeader(status)
	encoder := json.NewEncoder(c.w)
	return encoder.Encode(data)
}

func (c *Ctx) Text(status int, s string) error {
	c.w.WriteHeader(status)
	c.writeHeader("Content-Type", "text/plain; charset=utf-8")
	_, err := io.WriteString(c.w, s)
	return err
}

func (c *Ctx) Redirect(status int, url string) error {
	c.w.WriteHeader(status)
	http.Redirect(c.w, c.r, url, status)
	return nil
}

func (c *Ctx) File(status int, path string) error {
	c.w.WriteHeader(status)
	http.ServeFile(c.w, c.r, path)
	return nil
}

func (c *Ctx) Bytes(status int, contentType string, bytes []byte) error {
	c.w.WriteHeader(status)
	c.writeHeader("Content-Type", contentType)
	_, err := c.w.Write(bytes)
	return err
}

func (c *Ctx) Html(status int, s string) error {
	c.w.WriteHeader(status)
	c.writeHeader("Content-Type", "text/html; charset=utf-8")
	_, err := io.WriteString(c.w, s)
	return err
}
