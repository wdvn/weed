package weed

import (
	"fmt"
	"github.com/wdvn/weed/internal/core/http"
	std "net/http"
)

type App struct {
	router *http.Router
	sv     std.Server
}

func New() *App {
	return &App{
		router: http.NewRouter(),
		sv:     std.Server{},
	}
}

func (app *App) Serve(port string) error {
	app.sv.Addr = fmt.Sprintf("%s", port)
	app.sv.Handler = app.router
	return app.sv.ListenAndServe()
}

func (app *App) Use(middle http.MiddlewareFunc) {
	app.router.Use(middle)
}
