package weed

import (
	"fmt"
	"testing"
	"time"

	"github.com/wdvn/weed/core/http"
)

func TestApp_Serve(t *testing.T) {
	app := New()
	app.Use(func(next http.HandlerFunc) http.HandlerFunc {
		return func(ctx *http.Ctx) error {
			fmt.Printf("%v %s %s\n", time.Now(), ctx.Request().Method, ctx.Request().URL.Path)
			return next(ctx)
		}
	})
	app.router.GET("/", func(ctx *http.Ctx) error {
		return ctx.Text(200, "ahihi")
	})
	app.router.Static("/static", "./tmp")
	err := app.Serve(":8080")
	if err != nil {
		t.Error(err)
	}
}
