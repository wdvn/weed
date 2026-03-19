package weed

import (
	"github.com/wdvn/weed/internal/core/http"
	"testing"
)

func TestApp_Serve(t *testing.T) {
	app := New()
	app.router.GET("/", func(ctx *http.Ctx) error {
		return ctx.Text(200, "ahihi")
	})
	app.router.Static("/static", "./tmp")
	err := app.Serve(":8080")
	if err != nil {
		t.Error(err)
	}
}
