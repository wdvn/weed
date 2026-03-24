package rest

import "reflect"

// RouteMeta holds metadata for a registered route, useful for generating OpenAPI documentation.
type RouteMeta struct {
	Method   string
	Path     string
	ReqType  reflect.Type
	RespType reflect.Type
}
