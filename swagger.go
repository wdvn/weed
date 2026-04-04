package weed

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/wdvn/weed/core/meta"
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

// GenerateOpenAPI generates an OpenAPI 3.0 JSON specification from the registered routes.
func (app *App) GenerateOpenAPI() []byte {
	openapi := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":   "Weed App API",
			"version": "1.0.0",
		},
		"paths": make(map[string]interface{}),
	}

	paths := openapi["paths"].(map[string]interface{})

	// Helper to convert go type to openapi type
	var getOpenAPIType func(t reflect.Type) map[string]interface{}
	getOpenAPIType = func(t reflect.Type) map[string]interface{} {
		if t.Kind() == reflect.Ptr {
			return getOpenAPIType(t.Elem())
		}

		switch t.Kind() {
		case reflect.String:
			return map[string]interface{}{"type": "string"}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return map[string]interface{}{"type": "integer"}
		case reflect.Float32, reflect.Float64:
			return map[string]interface{}{"type": "number"}
		case reflect.Bool:
			return map[string]interface{}{"type": "boolean"}
		case reflect.Slice, reflect.Array:
			return map[string]interface{}{
				"type":  "array",
				"items": getOpenAPIType(t.Elem()),
			}
		case reflect.Struct:
			props := make(map[string]interface{})
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				jsonTag := field.Tag.Get("json")
				if jsonTag == "-" {
					continue
				}
				name := field.Name
				if jsonTag != "" {
					name = strings.Split(jsonTag, ",")[0]
				}
				props[name] = getOpenAPIType(field.Type)
			}
			return map[string]interface{}{
				"type":       "object",
				"properties": props,
			}
		default:
			return map[string]interface{}{"type": "string"}
		}
	}

	for _, m := range meta.All() {
		// Clean path for OpenAPI (e.g. /users/:id -> /users/{id})
		openApiPath := m.Path
		parts := strings.Split(openApiPath, "/")
		for i, p := range parts {
			if strings.HasPrefix(p, ":") {
				parts[i] = "{" + p[1:] + "}"
			}
		}
		openApiPath = strings.Join(parts, "/")

		if _, exists := paths[openApiPath]; !exists {
			paths[openApiPath] = make(map[string]interface{})
		}

		pathItem := paths[openApiPath].(map[string]interface{})
		methodLower := strings.ToLower(m.Method)

		operation := map[string]interface{}{
			"responses": map[string]interface{}{
				"200": map[string]interface{}{
					"description": "Successful response",
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": getOpenAPIType(m.RespType),
						},
					},
				},
			},
		}

		// Parse Request struct to generate parameters and requestBody
		var parameters []interface{}
		var bodyProps map[string]interface{}

		if m.ReqType != nil && m.ReqType.Kind() == reflect.Struct {
			bodyProps = make(map[string]interface{})

			for i := 0; i < m.ReqType.NumField(); i++ {
				field := m.ReqType.Field(i)

				// Handle Path parameters
				if pathTag := field.Tag.Get("path"); pathTag != "" {
					parameters = append(parameters, map[string]interface{}{
						"name":     pathTag,
						"in":       "path",
						"required": true,
						"schema":   getOpenAPIType(field.Type),
					})
					continue
				}

				// Handle Query parameters
				if queryTag := field.Tag.Get("query"); queryTag != "" {
					parameters = append(parameters, map[string]interface{}{
						"name":   queryTag,
						"in":     "query",
						"schema": getOpenAPIType(field.Type),
					})
					continue
				}

				// Handle Header parameters
				if headerTag := field.Tag.Get("header"); headerTag != "" {
					parameters = append(parameters, map[string]interface{}{
						"name":   headerTag,
						"in":     "header",
						"schema": getOpenAPIType(field.Type),
					})
					continue
				}

				// Otherwise, it's considered part of the JSON body
				jsonTag := field.Tag.Get("json")
				if jsonTag != "-" {
					name := field.Name
					if jsonTag != "" {
						name = strings.Split(jsonTag, ",")[0]
					}
					bodyProps[name] = getOpenAPIType(field.Type)
				}
			}
		}

		if len(parameters) > 0 {
			operation["parameters"] = parameters
		}

		if (m.Method == "POST" || m.Method == "PUT" || m.Method == "PATCH") && len(bodyProps) > 0 {
			operation["requestBody"] = map[string]interface{}{
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{
							"type":       "object",
							"properties": bodyProps,
						},
					},
				},
			}
		}
		pathItem[methodLower] = operation
	}

	b, _ := json.MarshalIndent(openapi, "", "  ")
	return b
}
