package weed

import (
	"fmt"
)

const swaggerUITemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <meta name="description" content="SwaggerUI" />
  <title>SwaggerUI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui.css" />
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-bundle.js" crossorigin></script>
<script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-standalone-preset.js" crossorigin></script>
<script>
  window.onload = () => {
    window.ui = SwaggerUIBundle({
      url: '%s',
      dom_id: '#swagger-ui',
      presets: [
        SwaggerUIBundle.presets.apis,
        SwaggerUIStandalonePreset
      ],
      layout: "StandaloneLayout",
    });
  };
</script>
</body>
</html>`

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
	html := fmt.Sprintf(swaggerUITemplate, jsonPath)

	// Serve the Swagger UI HTML
	app.GET(path, func(c *Ctx) error {
		return c.Html(200, html)
	})

	// Also serve on path + "/" to avoid missing trailing slash issues
	app.GET(path+"/", func(c *Ctx) error {
		return c.Html(200, html)
	})
}
