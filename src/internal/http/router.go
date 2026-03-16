package http

import (
	"net/http"
	"strings"
)

// Param represents a URL parameter (e.g., :id).
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

// HandlerFunc is a custom handler that includes URL parameters.
type HandlerFunc func(w http.ResponseWriter, r *http.Request, params Params)

// node represents a Radix Tree (Trie) node.
// We use a segment-based approach (splitting by '/') which is highly efficient
// and easy to implement for HTTP routing with parameters.
type node struct {
	part     string // e.g. "users", ":id", "*filepath"
	path     string // full path if it's a leaf node
	children []*node
	isWild   bool // true if it's a param (:id) or catch-all (*filepath)
	handler  HandlerFunc
}

// Router is an HTTP routing multiplexer using a Radix tree over path segments.
type Router struct {
	roots map[string]*node
}

// NewRouter creates a new Router.
func NewRouter() *Router {
	return &Router{
		roots: make(map[string]*node),
	}
}

// splitPath splits the path by '/' and ignores empty segments.
func splitPath(path string) []string {
	segments := strings.Split(path, "/")
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment != "" {
			parts = append(parts, segment)
			// Stop splitting if it's a catch-all wildcard
			if segment[0] == '*' {
				break
			}
		}
	}
	return parts
}

// Handle registers a new request handler with the given path and method.
func (r *Router) Handle(method string, path string, handler HandlerFunc) {
	parts := splitPath(path)
	if _, ok := r.roots[method]; !ok {
		r.roots[method] = &node{}
	}
	r.roots[method].insert(path, parts, 0, handler)
}

// GET is a shortcut for Handle(http.MethodGet, path, handler)
func (r *Router) GET(path string, handler HandlerFunc) {
	r.Handle(http.MethodGet, path, handler)
}

// POST is a shortcut for Handle(http.MethodPost, path, handler)
func (r *Router) POST(path string, handler HandlerFunc) {
	r.Handle(http.MethodPost, path, handler)
}

func (n *node) insert(path string, parts []string, height int, handler HandlerFunc) {
	if len(parts) == height {
		n.path = path
		n.handler = handler
		return
	}

	part := parts[height]
	var child *node
	for _, c := range n.children {
		if c.part == part {
			child = c
			break
		}
	}

	if child == nil {
		child = &node{
			part:   part,
			isWild: part[0] == ':' || part[0] == '*',
		}
		n.children = append(n.children, child)
	}
	child.insert(path, parts, height+1, handler)
}

func (n *node) search(parts []string, height int) *node {
	if len(parts) == height || strings.HasPrefix(n.part, "*") {
		if n.handler == nil {
			return nil
		}
		return n
	}

	part := parts[height]
	for _, child := range n.children {
		if child.part == part || child.isWild {
			result := child.search(parts, height+1)
			if result != nil {
				return result
			}
		}
	}
	return nil
}

// Search returns the matched handler and extracted URL parameters.
func (r *Router) Search(method string, path string) (HandlerFunc, Params) {
	root, ok := r.roots[method]
	if !ok {
		return nil, nil
	}

	searchParts := splitPath(path)
	n := root.search(searchParts, 0)
	if n != nil {
		parts := splitPath(n.path)
		var params Params
		for i, part := range parts {
			if part[0] == ':' {
				params = append(params, Param{
					Key:   part[1:],
					Value: searchParts[i],
				})
			}
			if part[0] == '*' && len(part) > 1 {
				params = append(params, Param{
					Key:   part[1:],
					Value: strings.Join(searchParts[i:], "/"),
				})
				break
			}
		}
		return n.handler, params
	}
	return nil, nil
}

// ServeHTTP makes the Router implement the http.Handler interface.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	handler, params := r.Search(req.Method, req.URL.Path)
	if handler != nil {
		handler(w, req, params)
	} else {
		http.NotFound(w, req)
	}
}
