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
