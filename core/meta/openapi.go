package meta

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

//go:embed ui.html
var SwaggerUITemplate string

// SwaggerUIHTML returns the Swagger UI HTML page pointing to the given openapi.json URL.
func SwaggerUIHTML(jsonURL string) string {
	return fmt.Sprintf(SwaggerUITemplate, jsonURL)
}

// GenerateOpenAPI generates an OpenAPI 3.0 JSON specification from all registered route metadata.
func GenerateOpenAPI() []byte {
	openapi := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":   "Weed App API",
			"version": "1.0.0",
		},
		"paths": make(map[string]interface{}),
	}

	paths := openapi["paths"].(map[string]interface{})

	for _, m := range All() {
		openApiPath := toOpenAPIPath(m.Path)

		if _, exists := paths[openApiPath]; !exists {
			paths[openApiPath] = make(map[string]interface{})
		}

		pathItem := paths[openApiPath].(map[string]interface{})
		operation := buildOperation(m)
		pathItem[strings.ToLower(m.Method)] = operation
	}

	b, _ := json.MarshalIndent(openapi, "", "  ")
	return b
}

// toOpenAPIPath converts a weed path to OpenAPI format.
// e.g. /users/:id -> /users/{id}, /static/*filepath -> /static/{filepath}
func toOpenAPIPath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ":") {
			parts[i] = "{" + p[1:] + "}"
		} else if strings.HasPrefix(p, "*") {
			parts[i] = "{" + p[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

// isStaticRoute returns true if the path contains a catch-all wildcard (*).
func isStaticRoute(path string) bool {
	return strings.Contains(path, "*")
}

// buildOperation constructs the full OpenAPI operation object for a single route.
func buildOperation(m RouteMeta) map[string]interface{} {
	operation := buildResponse(m)
	buildParameters(m, operation)
	buildRequestBody(m, operation)

	if m.Tag != "" {
		operation["tags"] = []string{m.Tag}
	}

	return operation
}

// buildResponse creates the base operation with the appropriate response schema.
func buildResponse(m RouteMeta) map[string]interface{} {
	if isStaticRoute(m.Path) {
		return buildStaticFileResponse()
	}
	if m.RespType != nil {
		return buildTypedResponse(m.RespType)
	}
	return buildArbitraryJSONResponse()
}

// buildStaticFileResponse creates an operation for static file download routes.
func buildStaticFileResponse() map[string]interface{} {
	return map[string]interface{}{
		"summary": "Static file download",
		"responses": map[string]interface{}{
			"200": map[string]interface{}{
				"description": "File download",
				"content": map[string]interface{}{
					"application/octet-stream": map[string]interface{}{
						"schema": map[string]interface{}{
							"type":   "string",
							"format": "binary",
						},
					},
				},
			},
			"404": map[string]interface{}{
				"description": "File not found",
			},
		},
		"parameters": []interface{}{
			map[string]interface{}{
				"name":     "filepath",
				"in":       "path",
				"required": true,
				"schema":   map[string]interface{}{"type": "string"},
			},
		},
	}
}

// buildTypedResponse creates an operation with a fully typed JSON response schema.
func buildTypedResponse(respType reflect.Type) map[string]interface{} {
	return map[string]interface{}{
		"responses": map[string]interface{}{
			"200": map[string]interface{}{
				"description": "Successful response",
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": reflectToSchema(respType),
					},
				},
			},
		},
	}
}

// buildArbitraryJSONResponse creates an operation with an arbitrary JSON response (empty schema).
func buildArbitraryJSONResponse() map[string]interface{} {
	return map[string]interface{}{
		"responses": map[string]interface{}{
			"200": map[string]interface{}{
				"description": "Successful response",
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{},
					},
				},
			},
		},
	}
}

// buildParameters extracts path, query, and header parameters from the request struct tags
// and adds them to the operation.
func buildParameters(m RouteMeta, operation map[string]interface{}) {
	if m.ReqType == nil || m.ReqType.Kind() != reflect.Struct {
		return
	}

	var parameters []interface{}

	for i := 0; i < m.ReqType.NumField(); i++ {
		field := m.ReqType.Field(i)

		if tag := field.Tag.Get("path"); tag != "" {
			parameters = append(parameters, map[string]interface{}{
				"name":     tag,
				"in":       "path",
				"required": true,
				"schema":   reflectToSchema(field.Type),
			})
		} else if tag := field.Tag.Get("query"); tag != "" {
			parameters = append(parameters, map[string]interface{}{
				"name":   tag,
				"in":     "query",
				"schema": reflectToSchema(field.Type),
			})
		} else if tag := field.Tag.Get("header"); tag != "" {
			parameters = append(parameters, map[string]interface{}{
				"name":   tag,
				"in":     "header",
				"schema": reflectToSchema(field.Type),
			})
		}
	}

	if len(parameters) == 0 {
		return
	}

	// Merge with existing parameters (e.g. static route already has filepath param)
	if existing, ok := operation["parameters"]; ok {
		operation["parameters"] = append(existing.([]interface{}), parameters...)
	} else {
		operation["parameters"] = parameters
	}
}

// buildRequestBody extracts JSON body fields from the request struct and adds
// a requestBody to the operation for POST/PUT/PATCH methods.
func buildRequestBody(m RouteMeta, operation map[string]interface{}) {
	if m.ReqType == nil || m.ReqType.Kind() != reflect.Struct {
		return
	}
	if m.Method != "POST" && m.Method != "PUT" && m.Method != "PATCH" {
		return
	}

	bodyProps := make(map[string]interface{})
	for i := 0; i < m.ReqType.NumField(); i++ {
		field := m.ReqType.Field(i)

		// Skip fields bound to path/query/header
		if field.Tag.Get("path") != "" || field.Tag.Get("query") != "" || field.Tag.Get("header") != "" {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		name := field.Name
		if jsonTag != "" {
			name = strings.Split(jsonTag, ",")[0]
		}
		bodyProps[name] = reflectToSchema(field.Type)
	}

	if len(bodyProps) == 0 {
		return
	}

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

// reflectToSchema converts a Go reflect.Type to an OpenAPI schema object.
func reflectToSchema(t reflect.Type) map[string]interface{} {
	if t == nil {
		return map[string]interface{}{}
	}
	if t.Kind() == reflect.Ptr {
		return reflectToSchema(t.Elem())
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
			"items": reflectToSchema(t.Elem()),
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
			props[name] = reflectToSchema(field.Type)
		}
		return map[string]interface{}{
			"type":       "object",
			"properties": props,
		}
	default:
		return map[string]interface{}{"type": "string"}
	}
}
