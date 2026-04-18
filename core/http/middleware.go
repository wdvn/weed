package http

type MiddlewareFunc func(next HandlerFunc) HandlerFunc
