package weed

import (
	"fmt"

	"github.com/wdvn/weed/core/meta"
)

// Swagger serves a Swagger UI for the given openapi.json data at the specified path.
// For example, if path is "/docs", the UI is available at "/docs" and the JSON at "/docs/openapi.json".
func (app *App) Swagger(path string, openapiJSON []byte) {
	// Normalize path by removing trailing slash if present
	if len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	jsonPath := path + "/openapi.json"

	// Serve the openapi.json file
	app.GET(jsonPath, func(c *Ctx) error {
		return c.Bytes(200, "application/json; charset=utf-8", openapiJSON)
	})

	// Generate the HTML with the injected JSON path
	html := meta.SwaggerUIHTML(jsonPath)

	// Serve the Swagger UI HTML
	app.GET(path, func(c *Ctx) error {
		return c.Html(200, html)
	})

	// Also serve on path + "/" to avoid missing trailing slash issues
	app.GET(path+"/", func(c *Ctx) error {
		return c.Html(200, html)
	})
}

// GenerateOpenAPI generates an OpenAPI 3.0 JSON specification from the registered routes.
func (app *App) GenerateOpenAPI() []byte {
	return meta.GenerateOpenAPI()
}

// SwaggerAutoServe is a convenience method that generates the OpenAPI spec and serves Swagger UI.
// Usage: app.SwaggerAutoServe("/docs")
func (app *App) SwaggerAutoServe(path string) {
	spec := meta.GenerateOpenAPI()

	if len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	jsonPath := path + "/openapi.json"

	app.GET(jsonPath, func(c *Ctx) error {
		return c.Bytes(200, "application/json; charset=utf-8", spec)
	})

	html := fmt.Sprintf(meta.SwaggerUITemplate, jsonPath)

	app.GET(path, func(c *Ctx) error {
		return c.Html(200, html)
	})

	app.GET(path+"/", func(c *Ctx) error {
		return c.Html(200, html)
	})
}
