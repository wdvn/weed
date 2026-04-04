package meta

import (
	"reflect"
	"sync"

	"github.com/wdvn/weed/core/ds/radix"
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
	mu    sync.RWMutex
	roots map[string]*radix.Tree[RouteMeta] // keyed by HTTP method (GET, POST, etc.)
)

func init() {
	roots = make(map[string]*radix.Tree[RouteMeta])
}

// merge updates existing metadata with non-zero fields from the incoming entry.
func merge(existing *RouteMeta, incoming RouteMeta) {
	if incoming.ReqType != nil {
		existing.ReqType = incoming.ReqType
	}
	if incoming.RespType != nil {
		existing.RespType = incoming.RespType
	}
	if incoming.Tag != "" {
		existing.Tag = incoming.Tag
	}
}

// Register adds or updates one or more RouteMeta entries in the radix tree.
// If a route with the same Method+Path already exists, its fields are merged
// (non-zero fields in the new entry override the existing ones).
// This allows Handle() to register basic metadata (method, path) first,
// and contract-based Mount() to enrich it later with ReqType/RespType/Tag.
func Register(metas ...RouteMeta) {
	mu.Lock()
	defer mu.Unlock()
	for i := range metas {
		m := &metas[i]
		tree, ok := roots[m.Method]
		if !ok {
			tree = radix.New[RouteMeta]()
			roots[m.Method] = tree
		}
		tree.Upsert(m.Path, *m, merge)
	}
}

// All returns all registered route metadata by traversing the radix trees.
func All() []RouteMeta {
	mu.RLock()
	defer mu.RUnlock()
	var result []RouteMeta
	for _, tree := range roots {
		result = append(result, tree.Collect()...)
	}
	return result
}

// Reset clears all registered metadata. Useful for testing.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	roots = make(map[string]*radix.Tree[RouteMeta])
}
