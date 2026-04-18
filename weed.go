package weed

import (
	"fmt"
	std "net/http"

	"github.com/wdvn/weed/core/driven/rest"
	"github.com/wdvn/weed/core/http"
	"github.com/wdvn/weed/core/meta"
)

// Ctx is an alias for http.Ctx so external projects can use weed.Ctx
type Ctx = http.Ctx

// HandlerFunc is an alias for the router handler function
type HandlerFunc = http.HandlerFunc

// MiddlewareFunc is an alias for the router middleware function
type MiddlewareFunc = http.MiddlewareFunc

// RouterGroup is an alias for the router group
type RouterGroup = http.RouterGroup

// Renderer is the interface for template renderers (mirrors echo.Renderer).
type Renderer = http.Renderer

// Response is the response wrapper that tracks Committed state (mirrors echo.Response).
type Response = http.Response

// HTTPError represents an HTTP error with a code and message (mirrors echo.HTTPError).
type HTTPError = http.HTTPError

// NewHTTPError creates a new HTTPError (mirrors echo.NewHTTPError).
var NewHTTPError = http.NewHTTPError

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

func (app *App) Use(middles ...http.MiddlewareFunc) {
	app.router.Use(middles...)
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

// SetRenderer sets the template renderer for all contexts served by this app.
func (app *App) SetRenderer(r Renderer) {
	app.router.SetRenderer(r)
}

// RoutesMeta returns all registered route metadata from the global registry.
func (app *App) RoutesMeta() []meta.RouteMeta {
	return meta.All()
}

func (app *App) AddService(name string, sv any, middleware ...MiddlewareFunc) error {
	_, err := rest.Mount(app.router.RouterGroup, name, sv, middleware...)
	return err
}

func (app *App) AddServiceToGroup(group *RouterGroup, name string, sv any, middleware ...MiddlewareFunc) error {
	_, err := rest.Mount(group, name, sv, middleware...)
	return err
}
