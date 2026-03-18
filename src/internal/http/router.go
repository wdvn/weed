package http

import (
	"net/http"
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
// LƯU Ý QUAN TRỌNG: Để đạt được "Zero Allocation", biến `params` được lấy từ sync.Pool.
// Vòng đời của `params` chỉ hợp lệ trong lúc function này chạy.
// TUYỆT ĐỐI KHÔNG truyền `params` sang một Goroutine khác. Nếu cần, hãy copy nó.
type HandlerFunc func(ctx *Ctx) error

// node represents a Radix Tree (Trie) node optimized for zero allocations.
type node struct {
	part        string
	paramKey    string // Lưu sẵn key cho :param và *catchall để tránh cắt chuỗi lúc runtime
	children    []*node
	isWild      bool // true nếu là param (:id) hoặc catch-all (*filepath)
	isParam     bool // true nếu là param (:id)
	isCatch     bool // true nếu là catch-all (*filepath)
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
	child.insert(path, parts, height+1, handler)
}

// search duyệt Tree hoàn toàn không cấp phát RAM (Zero Allocation)
func (n *node) search(path string, params *Params) HandlerFunc {
	// Bỏ qua các dấu "/" ở đầu
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
		nextPath = path[idx:] // nextPath bắt đầu bằng "/", sẽ được xoá ở vòng lặp sau
	}

	// 1. Ưu tiên Exact match (Khớp chính xác)
	for _, child := range n.children {
		if child.part == p && !child.isWild {
			if h := child.search(nextPath, params); h != nil {
				return h
			}
			break
		}
	}

	// 2. Wildcard match (Tham số hoặc Catch-all)
	for _, child := range n.children {
		if child.isParam {
			origLen := len(*params)
			*params = append(*params, Param{Key: child.paramKey, Value: p})
			if h := child.search(nextPath, params); h != nil {
				return h
			}
			// Backtrack lại nếu nhánh này không tìm thấy đích
			*params = (*params)[:origLen]
		} else if child.isCatch {
			// Trả về toàn bộ phần còn lại (không bao gồm "/" ở đầu do đã xử lý bên trên)
			*params = append(*params, Param{Key: child.paramKey, Value: path})
			if child.handler != nil {
				return child.handler
			}
		}
	}

	return nil
}

// Router is an HTTP routing multiplexer optimized for high concurrency and low RAM usage.
type Router struct {
	roots      map[string]*node
	paramsPool sync.Pool
}

// NewRouter creates a new Router.
func NewRouter() *Router {
	return &Router{
		roots: make(map[string]*node),
		paramsPool: sync.Pool{
			New: func() any {
				p := make(Params, 0, 20)
				return &p
			},
		},
	}
}

// splitPath splits the path by '/' and ignores empty segments.
// Hàm này chỉ chạy lúc khởi tạo (Handle), nên Allocation ở đây là hoàn toàn OK.
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

// ServeHTTP makes the Router implement the http.Handler interface.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	root, ok := r.roots[req.Method]
	if !ok {
		http.NotFound(w, req)
		return
	}

	// Mượn Params slice từ sync.Pool
	p := r.paramsPool.Get().(*Params)
	*p = (*p)[:0] // Reset chiều dài về 0 nhưng giữ nguyên memory capacity

	// Duyệt Radix Tree để tìm kết quả
	handler := root.search(req.URL.Path, p)
	if handler != nil {
		ctx := NewCtx(w, req, *p...)
		err := handler(ctx)
		if err != nil {
			_ = ctx.Text(http.StatusInternalServerError, err.Error())
		}
		r.paramsPool.Put(p)
		return
	}

	r.paramsPool.Put(p)
	http.NotFound(w, req)
}
