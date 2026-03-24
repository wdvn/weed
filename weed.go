package weed

import (
	"fmt"
	std "net/http"

	"github.com/wdvn/weed/core/http"
)

// Ctx is an alias for http.Ctx so external projects can use weed.Ctx
type Ctx = http.Ctx

// HandlerFunc is an alias for the router handler function
type HandlerFunc = http.HandlerFunc

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

func (app *App) Use(middle http.MiddlewareFunc) {
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
func (app *App) Group(prefix string, middlewares ...http.MiddlewareFunc) *http.RouterGroup {
	return app.router.Group(prefix, middlewares...)
}

// Static serves static files from the given root directory
func (app *App) Static(prefix string, root string) {
	app.router.Static(prefix, root)
}
