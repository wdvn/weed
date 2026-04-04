package meta

import (
	"reflect"
	"sync"
)

// RouteMeta holds metadata for a registered route, useful for generating OpenAPI documentation.
type RouteMeta struct {
	Method   string
	Path     string
	ReqType  reflect.Type
	RespType reflect.Type
	Tag      string // using for service name => open api tag
}

var (
	mu     sync.RWMutex
	routes []RouteMeta
)

// Register adds or updates one or more RouteMeta entries in the global registry.
// If a route with the same Method+Path already exists, its fields are merged
// (non-zero fields in the new entry override the existing ones).
// This allows Handle() to register basic metadata (method, path) first,
// and contract-based Mount() to enrich it later with ReqType/RespType/Tag.
func Register(metas ...RouteMeta) {
	mu.Lock()
	defer mu.Unlock()
	for _, m := range metas {
		if idx := findRoute(m.Method, m.Path); idx >= 0 {
			// Merge: only override with non-zero values
			if m.ReqType != nil {
				routes[idx].ReqType = m.ReqType
			}
			if m.RespType != nil {
				routes[idx].RespType = m.RespType
			}
			if m.Tag != "" {
				routes[idx].Tag = m.Tag
			}
		} else {
			routes = append(routes, m)
		}
	}
}

// All returns a copy of all registered route metadata.
func All() []RouteMeta {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]RouteMeta, len(routes))
	copy(out, routes)
	return out
}

// Reset clears all registered metadata. Useful for testing.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	routes = routes[:0]
}

// findRoute returns the index of the route with the given method and path, or -1 if not found.
// Must be called with mu held.
func findRoute(method, path string) int {
	for i := range routes {
		if routes[i].Method == method && routes[i].Path == path {
			return i
		}
	}
	return -1
}
