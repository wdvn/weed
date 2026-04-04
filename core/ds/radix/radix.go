package radix

import "strings"

// Param represents a URL parameter captured during tree search (e.g., :id → {Key:"id", Value:"123"}).
type Param struct {
	Key   string
	Value string
}

// Params is a slice of URL parameters.
type Params []Param

// Get returns the value of the parameter with the given key.
func (ps Params) Get(name string) string {
	for i := range ps {
		if ps[i].Key == name {
			return ps[i].Value
		}
	}
	return ""
}

// node represents a radix tree (trie) node.
type node[T any] struct {
	part     string
	paramKey string // Pre-computed key for :param and *catchall to avoid string slicing at runtime
	children []*node[T]
	isWild   bool // true if it is a param (:id) or catch-all (*filepath)
	isParam  bool // true if it is a param (:id)
	isCatch  bool // true if it is a catch-all (*filepath)
	value    *T   // non-nil when this node is a route endpoint
}

func (n *node[T]) insert(parts []string, height int, value *T) {
	if len(parts) == height {
		n.value = value
		return
	}

	part := parts[height]
	var child *node[T]
	for _, c := range n.children {
		if c.part == part {
			child = c
			break
		}
	}

	if child == nil {
		child = &node[T]{
			part:    part,
			isWild:  part[0] == ':' || part[0] == '*',
			isParam: part[0] == ':',
			isCatch: part[0] == '*',
		}
		if child.isParam || child.isCatch {
			child.paramKey = part[1:] // Pre-slice param name during initialization
		}
		n.children = append(n.children, child)
	}
	child.insert(parts, height+1, value)
}

// search traverses the tree with zero heap allocation.
func (n *node[T]) search(path string, params *Params) *node[T] {
	// Skip leading "/" characters
	for len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	if len(path) == 0 {
		return n
	}

	var p, nextPath string
	idx := strings.IndexByte(path, '/')
	if idx == -1 {
		p = path
		nextPath = ""
	} else {
		p = path[:idx]
		nextPath = path[idx:] // nextPath starts with "/", removed in the next recursion
	}

	// 1. Prioritize exact match
	for _, child := range n.children {
		if child.part == p && !child.isWild {
			if h := child.search(nextPath, params); h != nil {
				return h
			}
			break
		}
	}

	// 2. Wildcard match (Parameter or Catch-all)
	for _, child := range n.children {
		if child.isParam {
			origLen := len(*params)
			// Manually increase slice length to avoid `append` causing heap allocations
			*params = (*params)[:origLen+1]
			(*params)[origLen] = Param{Key: child.paramKey, Value: p}
			if h := child.search(nextPath, params); h != nil {
				return h
			}
			// Backtrack if this branch does not find a destination
			*params = (*params)[:origLen]
		} else if child.isCatch {
			// Return the entire remaining part
			origLen := len(*params)
			*params = (*params)[:origLen+1]
			(*params)[origLen] = Param{Key: child.paramKey, Value: path}
			if child.value != nil {
				return child
			}
		}
	}

	return nil
}

// collect traverses all nodes and appends stored values to the result slice.
func (n *node[T]) collect(result *[]T) {
	if n.value != nil {
		*result = append(*result, *n.value)
	}
	for _, child := range n.children {
		child.collect(result)
	}
}

// SplitPath splits a URL path into segments by '/' and ignores empty segments.
// Catch-all segments (*filepath) stop further splitting.
func SplitPath(path string) []string {
	segments := strings.Split(path, "/")
	parts := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg != "" {
			parts = append(parts, seg)
			if seg[0] == '*' {
				break
			}
		}
	}
	return parts
}

// Tree is a generic radix tree (trie) for storing values keyed by URL paths.
// T is the type of value stored at each route endpoint.
type Tree[T any] struct {
	root *node[T]
}

// New creates a new empty Tree.
func New[T any]() *Tree[T] {
	return &Tree[T]{root: &node[T]{}}
}

// Insert sets the value at the given path, overwriting any existing value.
func (t *Tree[T]) Insert(path string, value T) {
	parts := SplitPath(path)
	t.root.insert(parts, 0, &value)
}

// Upsert inserts a new value or updates an existing one using the merge function.
// If the path already has a value, merge(existing, incoming) is called instead of overwriting.
func (t *Tree[T]) Upsert(path string, value T, merge func(existing *T, incoming T)) {
	parts := SplitPath(path)

	// Navigate to the target node, creating nodes as needed
	n := t.root
	for height := 0; height < len(parts); height++ {
		part := parts[height]
		var child *node[T]
		for _, c := range n.children {
			if c.part == part {
				child = c
				break
			}
		}
		if child == nil {
			child = &node[T]{
				part:    part,
				isWild:  part[0] == ':' || part[0] == '*',
				isParam: part[0] == ':',
				isCatch: part[0] == '*',
			}
			if child.isParam || child.isCatch {
				child.paramKey = part[1:]
			}
			n.children = append(n.children, child)
		}
		n = child
	}

	// At the target node: merge or set
	if n.value != nil {
		merge(n.value, value)
	} else {
		n.value = &value
	}
}

// Search finds a value at the given path, populating params for :param and *catch-all segments.
// Returns a pointer to the value and true if found, nil and false otherwise.
func (t *Tree[T]) Search(path string, params *Params) (*T, bool) {
	n := t.root.search(path, params)
	if n != nil && n.value != nil {
		return n.value, true
	}
	return nil, false
}

// Collect returns all stored values by DFS traversal of the tree.
func (t *Tree[T]) Collect() []T {
	var result []T
	t.root.collect(&result)
	return result
}
