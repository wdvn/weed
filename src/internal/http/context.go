package http

import "net/http"

type Context interface {
	Request() *http.Request
	Writer() *http.ResponseWriter
	Query(key string) string

	Json(data any) error
	Text(s string) error
	File(path string) error
	Bytes(bytes []string) error
	Html(s string) error
}
