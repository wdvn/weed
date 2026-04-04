package http

import (
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/wdvn/weed/core/ds/radix"
	"github.com/wdvn/weed/core/meta"
)

// Param is an alias for radix.Param so existing code can use http.Param.
type Param = radix.Param

// Params is an alias for radix.Params so existing code can use http.Params.
type Params = radix.Params

// HandlerFunc is a custom handler that includes URL parameters.
// IMPORTANT NOTE: To achieve "Zero Allocation", the `ctx` variable is fetched from sync.Pool.
// The lifecycle of `ctx` is only valid while this function is running.
// DO NOT pass `ctx` to another Goroutine. If needed, please copy it.
type HandlerFunc func(ctx *Ctx) error

func wrapHandler(handler HandlerFunc, middlewares ...MiddlewareFunc) HandlerFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// RouterGroup represents a group of routes that share a common prefix and middlewares.
type RouterGroup struct {
	prefix      string
	middlewares []MiddlewareFunc
	router      *Router
}

// Prefix returns the full accumulated prefix of this group.
func (g *RouterGroup) Prefix() string {
	return g.prefix
}

// Group creates a new RouterGroup with the given prefix and middlewares.
func (g *RouterGroup) Group(prefix string, middlewares ...MiddlewareFunc) *RouterGroup {
	newMiddlewares := make([]MiddlewareFunc, 0, len(g.middlewares)+len(middlewares))
	newMiddlewares = append(newMiddlewares, g.middlewares...)
	newMiddlewares = append(newMiddlewares, middlewares...)

	return &RouterGroup{
		prefix:      g.prefix + prefix,
		middlewares: newMiddlewares,
		router:      g.router,
	}
}

// Use adds middlewares to the group.
func (g *RouterGroup) Use(middlewares ...MiddlewareFunc) {
	g.middlewares = append(g.middlewares, middlewares...)
}

// Router is an HTTP routing multiplexer optimized for high concurrency and low RAM usage.
type Router struct {
	*RouterGroup
	roots    map[string]*radix.Tree[HandlerFunc]
	ctxPool  sync.Pool
	renderer Renderer
}

// NewRouter creates a new Router.
func NewRouter() *Router {
	r := &Router{
		roots: make(map[string]*radix.Tree[HandlerFunc]),
		ctxPool: sync.Pool{
			New: func() any {
				// Pre-allocate Ctx with a params slice of capacity 20 to ensure zero allocation
				ctx := &Ctx{
					params: make(Params, 0, 20),
				}
				return ctx
			},
		},
	}
	r.RouterGroup = &RouterGroup{
		prefix: "",
		router: r,
	}
	return r
}

// Handle registers a new request handler with the given path and method.
func (g *RouterGroup) Handle(method string, path string, handler HandlerFunc, mw ...MiddlewareFunc) {
	fullPath := g.prefix + path
	if _, ok := g.router.roots[method]; !ok {
		g.router.roots[method] = radix.New[HandlerFunc]()
	}
	// Wrap the handler with all registered middlewares for this group
	combineMW := slices.Clone(g.middlewares)
	combineMW = append(combineMW, mw...)
	wrappedHandler := wrapHandler(handler, combineMW...)
	g.router.roots[method].Insert(fullPath, wrappedHandler)

	// Register basic route metadata so all routes are visible for OpenAPI generation
	meta.Register(meta.RouteMeta{
		Method: method,
		Path:   fullPath,
	})
}

// SetRenderer sets the template renderer used for all Ctx.Render() calls.
func (r *Router) SetRenderer(renderer Renderer) {
	r.renderer = renderer
}

// ServeHTTP makes the Router implement the http.Handler interface.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	tree, ok := r.roots[req.Method]
	if !ok {
		http.NotFound(w, req)
		return
	}

	// Borrow Ctx from sync.Pool instead of just Params
	ctx := r.ctxPool.Get().(*Ctx)
	resp := newResponse(w)
	ctx.r = req
	ctx.resp = resp
	ctx.w = resp // underlying writer is the Response wrapper
	ctx.cx = req.Context()
	ctx.params = ctx.params[:0] // Reset length to 0 but keep memory capacity
	ctx.store = nil             // reset per-request store
	ctx.renderer = r.renderer   // inject renderer
	// Traverse Radix Tree to find the result, pass params pointer to search
	handler, found := tree.Search(req.URL.Path, &ctx.params)
	if found {
		err := (*handler)(ctx)
		if err != nil {
			_ = ctx.Text(500, "Internal Server Error")
		}
		// Reset pointers to avoid memory leak if old request/writer are retained
		ctx.r = nil
		ctx.w = nil
		ctx.resp = nil
		ctx.cx = nil
		r.ctxPool.Put(ctx) // Return Ctx to Pool after finishing
		return
	}

	ctx.r = nil
	ctx.w = nil
	ctx.resp = nil
	ctx.cx = nil
	r.ctxPool.Put(ctx)
	http.NotFound(w, req)
}

//--------------METHOD----------------

// GET is a shortcut for Handle(http.MethodGet, path, handler)
func (g *RouterGroup) GET(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	g.Handle(http.MethodGet, path, handler, middlewares...)
}

// POST is a shortcut for Handle(http.MethodPost, path, handler)
func (g *RouterGroup) POST(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	g.Handle(http.MethodPost, path, handler, middlewares...)
}

// PUT is a shortcut for Handle(http.MethodPut, path, handler)
func (g *RouterGroup) PUT(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	g.Handle(http.MethodPut, path, handler, middlewares...)
}

// DELETE is a shortcut for Handle(http.MethodDelete, path, handler)
func (g *RouterGroup) DELETE(path string, handler HandlerFunc, middlewares ...MiddlewareFunc) {
	g.Handle(http.MethodDelete, path, handler, middlewares...)
}

func (g *RouterGroup) Static(prefix string, root string) {
	// Remove trailing slash for normalization
	prefix = strings.TrimSuffix(prefix, "/")
	fullPrefix := g.prefix + prefix

	// Use http.StripPrefix to remove prefix from URL before passing to FileServer.
	// Without this line, a request for "/static/css/style.css" would be searched by FileServer
	// in the directory "root/static/css/style.css" instead of "root/css/style.css".
	fileServer := http.StripPrefix(fullPrefix, http.FileServer(http.Dir(root)))

	// Register catch-all route to handle all requests starting with the prefix
	g.GET(prefix+"/*filepath", func(ctx *Ctx) error {
		fileServer.ServeHTTP(ctx.w, ctx.r)
		return nil
	})
}
