package weed

import (
	"encoding/json"
	"fmt"
	std "net/http"
	"reflect"
	"strings"

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
	router     *http.Router
	sv         std.Server
	routesMeta []rest.RouteMeta
}

func New() *App {
	r := http.NewRouter()
	return &App{
		router:     r,
		sv:         std.Server{Handler: r},
		routesMeta: make([]rest.RouteMeta, 0),
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

	for _, meta := range app.routesMeta {
		// Clean path for OpenAPI (e.g. /users/:id -> /users/{id})
		openApiPath := meta.Path
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
		methodLower := strings.ToLower(meta.Method)

		operation := map[string]interface{}{
			"responses": map[string]interface{}{
				"200": map[string]interface{}{
					"description": "Successful response",
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": getOpenAPIType(meta.RespType),
						},
					},
				},
			},
		}

		// Parse Request struct to generate parameters and requestBody
		var parameters []interface{}
		var bodyProps map[string]interface{}

		if meta.ReqType != nil && meta.ReqType.Kind() == reflect.Struct {
			bodyProps = make(map[string]interface{})

			for i := 0; i < meta.ReqType.NumField(); i++ {
				field := meta.ReqType.Field(i)

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

		if (meta.Method == "POST" || meta.Method == "PUT" || meta.Method == "PATCH") && len(bodyProps) > 0 {
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

func (app *App) AddService(name string, sv any) error {
	metas, err := rest.Mount(app.router.RouterGroup, name, sv)
	if err != nil {
		return err
	}
	app.routesMeta = append(app.routesMeta, metas...)
	return nil
}
