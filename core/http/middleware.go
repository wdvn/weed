package http

type MiddlewareFunc func(HandlerFunc) HandlerFunc
