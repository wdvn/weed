package weed

import (
	"fmt"
	std "net/http"

	"github.com/wdvn/weed/core/driven/rest"
	"github.com/wdvn/weed/core/http"
)

// Ctx is an alias for http.Ctx so external projects can use weed.Ctx
type Ctx = http.Ctx

// HandlerFunc is an alias for the router handler function
type HandlerFunc = http.HandlerFunc

// MiddlewareFunc is an alias for the router middleware function
type MiddlewareFunc = http.MiddlewareFunc

// RouterGroup is an alias for the router group
type RouterGroup = http.RouterGroup

type App struct {
	router *http.Router
	sv     std.Server
}

func New() *App {
	r := http.NewRouter()
	return &App{
		router: r,
		sv:     std.Server{Handler: r},
	}
}

func (app *App) Serve(port string) error {
	app.sv.Addr = fmt.Sprintf("%s", port)
	return app.sv.ListenAndServe()
}

func (app *App) Use(middle MiddlewareFunc) {
	app.router.Use(middle)
}

// GET is a shortcut for router.GET
func (app *App) GET(path string, handler HandlerFunc) {
	app.router.GET(path, handler)
}

// POST is a shortcut for router.POST
func (app *App) POST(path string, handler HandlerFunc) {
	app.router.POST(path, handler)
}

// PUT is a shortcut for router.PUT
func (app *App) PUT(path string, handler HandlerFunc) {
	app.router.PUT(path, handler)
}

// DELETE is a shortcut for router.DELETE
func (app *App) DELETE(path string, handler HandlerFunc) {
	app.router.DELETE(path, handler)
}

// Group creates a new router group
func (app *App) Group(prefix string, middlewares ...MiddlewareFunc) *RouterGroup {
	return app.router.Group(prefix, middlewares...)
}

// Static serves static files from the given root directory
func (app *App) Static(prefix string, root string) {
	app.router.Static(prefix, root)
}

// Register maps an implementation of a contract to the root router group.
// It iterates through the service methods and automatically binds routes based on RouteDescriptor requests.
//
// Deprecated: Consider using RegisterInterface for stronger contract-driven development.
func (app *App) Register(service any) error {
	return rest.Register(app.router.RouterGroup, service)
}

// RegisterInterface registers routes based on an interface definition.
// This allows true contract-driven design where the interface acts as the router contract.
// T must be an interface type, and service must be an implementation of T.
// Note: Go does not support generic methods on structs, so this is a standalone function.
func RegisterInterface[T any](app *App, service T) error {
	return rest.RegisterInterface[T](app.router.RouterGroup, service)
}

// RegisterGroupInterface registers routes based on an interface definition to a specific RouterGroup.
func RegisterGroupInterface[T any](group *RouterGroup, service T) error {
	return rest.RegisterInterface[T](group, service)
}
