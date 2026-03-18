package http

type MiddlewareFunc func(ctx *Ctx) error
