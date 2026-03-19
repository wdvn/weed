package http

import (
	"net/http"
	"slices"
	"strings"
	"sync"
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
// IMPORTANT NOTE: To achieve "Zero Allocation", the `ctx` variable is fetched from sync.Pool.
// The lifecycle of `ctx` is only valid while this function is running.
// DO NOT pass `ctx` to another Goroutine. If needed, please copy it.
type HandlerFunc func(ctx *Ctx) error

// node represents a Radix Tree (Trie) node optimized for zero allocations.
type node struct {
	part        string
	paramKey    string // Pre-computed key for :param and *catchall to avoid string slicing at runtime
	children    []*node
	isWild      bool // true if it is a param (:id) or catch-all (*filepath)
	isParam     bool // true if it is a param (:id)
	isCatch     bool // true if it is a catch-all (*filepath)
	handler     HandlerFunc
	middlewares []MiddlewareFunc
}

func (n *node) insert(path string, parts []string, height int, handler HandlerFunc) {
	if len(parts) == height {
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
			part:        part,
			isWild:      part[0] == ':' || part[0] == '*',
			isParam:     part[0] == ':',
			isCatch:     part[0] == '*',
			middlewares: slices.Clone(n.middlewares),
		}
		if child.isParam || child.isCatch {
			child.paramKey = part[1:] // Pre-slice param name during Router initialization
		}
		n.children = append(n.children, child)
	}
	child.insert(path, parts, height+1, handler)
}

// search traverses the Tree with zero RAM allocation (Zero Allocation)
func (n *node) search(path string, params *Params) HandlerFunc {
	// Skip leading "/" characters
	for len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	if len(path) == 0 {
		return n.handler
	}

	var p, nextPath string
	idx := strings.IndexByte(path, '/')
	if idx == -1 {
		p = path
		nextPath = ""
	} else {
		p = path[:idx]
		nextPath = path[idx:] // nextPath starts with "/", which will be removed in the next iteration
	}

	// 1. Prioritize Exact match
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
			// Return the entire remaining part (excluding leading "/" as handled above)
			origLen := len(*params)
			*params = (*params)[:origLen+1]
			(*params)[origLen] = Param{Key: child.paramKey, Value: path}
			if child.handler != nil {
				return child.handler
			}
		}
	}

	return nil
}

func (n *node) use(middle MiddlewareFunc) {
	n.middlewares = append(n.middlewares, middle)
}

// Router is an HTTP routing multiplexer optimized for high concurrency and low RAM usage.
type Router struct {
	roots       map[string]*node
	ctxPool     sync.Pool
	middlewares []MiddlewareFunc
}

// NewRouter creates a new Router.
func NewRouter() *Router {
	return &Router{
		roots: make(map[string]*node),
		ctxPool: sync.Pool{
			New: func() any {
				// Pre-allocate Ctx with a params slice of capacity 20 to ensure zero allocation
				return &Ctx{
					params: make(Params, 0, 20),
				}
			},
		},
	}
}

// splitPath splits the path by '/' and ignores empty segments.
// This function only runs during initialization (Handle), so Allocation here is completely OK.
func splitPath(path string) []string {
	segments := strings.Split(path, "/")
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment != "" {
			parts = append(parts, segment)
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
		r.roots[method] = &node{
			middlewares: r.middlewares,
		}
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

// PUT is a shortcut for Handle(http.MethodPut, path, handler)
func (r *Router) PUT(path string, handler HandlerFunc) {
	r.Handle(http.MethodPut, path, handler)
}

// DELETE is a shortcut for Handle(http.MethodDelete, path, handler)
func (r *Router) DELETE(path string, handler HandlerFunc) {
	r.Handle(http.MethodDelete, path, handler)
}

// ServeHTTP makes the Router implement the http.Handler interface.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	root, ok := r.roots[req.Method]
	if !ok {
		http.NotFound(w, req)
		return
	}

	// Borrow Ctx from sync.Pool instead of just Params
	ctx := r.ctxPool.Get().(*Ctx)
	ctx.r = req
	ctx.w = w
	ctx.cx = req.Context()
	ctx.params = ctx.params[:0] // Reset length to 0 but keep memory capacity
	// Traverse Radix Tree to find the result, pass params pointer to search
	handler := root.search(req.URL.Path, &ctx.params)
	if handler != nil {
		err := handler(ctx) // Ignore error in handler based on prototype (can be customized later)
		if err != nil {
			_ = ctx.Text(500, "Internal Server Error")
		}
		// Reset pointers to avoid memory leak if old request/writer are retained
		ctx.r = nil
		ctx.w = nil
		ctx.cx = nil
		r.ctxPool.Put(ctx) // Return Ctx to Pool after finishing
		return
	}

	ctx.r = nil
	ctx.w = nil
	ctx.cx = nil
	r.ctxPool.Put(ctx)
	http.NotFound(w, req)
}

func (r *Router) Static(prefix string, root string) {
	// Remove trailing slash for normalization
	prefix = strings.TrimSuffix(prefix, "/")

	// Use http.StripPrefix to remove prefix from URL before passing to FileServer.
	// Without this line, a request for "/static/css/style.css" would be searched by FileServer
	// in the directory "root/static/css/style.css" instead of "root/css/style.css".
	fileServer := http.StripPrefix(prefix, http.FileServer(http.Dir(root)))

	// Register catch-all route to handle all requests starting with the prefix
	r.GET(prefix+"/*filepath", func(ctx *Ctx) error {
		fileServer.ServeHTTP(ctx.w, ctx.r)
		return nil
	})
}

func (r *Router) Use(middle MiddlewareFunc) {
	for _, node := range r.roots {
		node.use(middle)
	}
}
