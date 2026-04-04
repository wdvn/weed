package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	weedhttp "github.com/wdvn/weed/core/http"
	"github.com/wdvn/weed/core/meta"
)

// ContractError allows handlers to return specific HTTP status codes and messages.
type ContractError interface {
	error
	StatusCode() int
}

type defaultError struct {
	status  int
	message string
}

func (e *defaultError) Error() string   { return e.message }
func (e *defaultError) StatusCode() int { return e.status }

// NewError creates a new ContractError with the specified status code and message.
func NewError(status int, message string) ContractError {
	return &defaultError{status: status, message: message}
}

// Handler is a generic wrapper that converts a strongly-typed contract method into a weedhttp.HandlerFunc.
// It parses the JSON request body (if any), executes the handler, and writes the JSON response.
func Handler[Req any, Resp any](h func(context.Context, *Req) (*Resp, error)) weedhttp.HandlerFunc {
	return func(c *weedhttp.Ctx) error {
		var req Req

		// Attempt to parse JSON body for methods like POST, PUT, PATCH
		method := c.Request().Method
		if method == "POST" || method == "PUT" || method == "PATCH" {
			if c.Request().Body != nil {
				if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
					return c.JSON(400, map[string]string{"error": "invalid request body: " + err.Error()})
				}
			}
		}

		// Bind path, query, and header parameters based on struct tags
		reqVal := reflect.ValueOf(&req)
		if err := bindRequestTags(c, reqVal); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request parameters: " + err.Error()})
		}

		// Execute the strongly-typed handler
		resp, err := h(c.Request().Context(), &req)
		if err != nil {
			var ce ContractError
			if errors.As(err, &ce) {
				return c.JSON(ce.StatusCode(), map[string]string{"error": ce.Error()})
			}
			return c.JSON(500, map[string]string{"error": "internal server error"})
		}

		// Return the response as JSON
		return c.JSON(200, resp)
	}
}

// Mount registers routes based on an interface definition.
// T must be an interface type. service must be an implementation of T.
// This allows true contract-driven design where the interface acts as the router contract.
func Mount(router *weedhttp.RouterGroup, name string, service any, middleware ...weedhttp.MiddlewareFunc) ([]RouteMeta, error) {
	var metas []RouteMeta
	svcType := reflect.TypeOf(service)
	svcVal := reflect.ValueOf(service)

	if svcType.Kind() != reflect.Ptr || svcVal.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("service must be a pointer to a struct")
	}

	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	errType := reflect.TypeOf((*error)(nil)).Elem()
	group := router.Group(fmt.Sprintf("/%s", name))
	for i := 0; i < svcType.NumMethod(); i++ {
		method := svcType.Method(i)
		mType := method.Type

		// A valid contract method must have 3 arguments (receiver, context, request)
		// and 2 return values (response, error)
		if mType.NumIn() != 3 || mType.NumOut() != 2 {
			continue
		}

		// Check parameter types
		if !mType.In(1).Implements(ctxType) {
			continue
		}

		reqType := mType.In(2)
		if reqType.Kind() != reflect.Ptr {
			continue
		}

		// Check return types
		if !mType.Out(1).Implements(errType) {
			continue
		}

		// We have a match! Let's instantiate the request to get the method and path
		reqInstance := reflect.New(reqType.Elem()).Interface()
		httpMethod := getHttpMethod(reqInstance)
		httpPath := fmt.Sprintf("/%s", toSnakeCase(method.Name))

		if httpMethod == "" || httpPath == "" {
			continue // Skip if the route is invalid
		}

		// Create a dynamic HandlerFunc that bridges the weedhttp.Ctx to the reflect method call
		handler := createDynamicHandlerFromValue(svcVal, method.Func, reqType)

		// Register with the router
		group.Handle(httpMethod, httpPath, handler, middleware...)

		metas = append(metas, RouteMeta{
			Method:   httpMethod,
			Path:     group.Prefix() + httpPath,
			ReqType:  reqType.Elem(),
			RespType: mType.Out(0).Elem(), // Pointer to response
			Tag:      name,
		})
	}

	meta.Register(metas...)
	return metas, nil
}

// bindRequestTags inspects the request struct and sets fields based on `path`, `query`, and `header` tags
func bindRequestTags(c *weedhttp.Ctx, reqVal reflect.Value) error {
	if reqVal.Kind() == reflect.Ptr {
		reqVal = reqVal.Elem()
	}

	if reqVal.Kind() != reflect.Struct {
		return nil
	}

	reqType := reqVal.Type()
	for i := 0; i < reqType.NumField(); i++ {
		field := reqType.Field(i)
		fieldVal := reqVal.Field(i)

		if !fieldVal.CanSet() {
			continue
		}

		// Check for path param tag
		if tag := field.Tag.Get("path"); tag != "" {
			val := c.Param(tag)
			if val != "" {
				if err := setFieldValue(fieldVal, val); err != nil {
					return fmt.Errorf("failed to bind path param %s: %w", tag, err)
				}
			}
		}

		// Check for query param tag
		if tag := field.Tag.Get("query"); tag != "" {
			val := c.Query(tag)
			if val != "" {
				if err := setFieldValue(fieldVal, val); err != nil {
					return fmt.Errorf("failed to bind query param %s: %w", tag, err)
				}
			}
		}

		// Check for header param tag
		if tag := field.Tag.Get("header"); tag != "" {
			val := c.Request().Header.Get(tag)
			if val != "" {
				if err := setFieldValue(fieldVal, val); err != nil {
					return fmt.Errorf("failed to bind header param %s: %w", tag, err)
				}
			}
		}
	}
	return nil
}

// setFieldValue converts a string to the target field type and sets it
func setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(uintVal)
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(floatVal)
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(boolVal)
	default:
		return fmt.Errorf("unsupported field type: %v", field.Kind())
	}
	return nil
}

// createDynamicHandlerFromValue creates a weedhttp.HandlerFunc from an already bound method value (for Interfaces)
func createDynamicHandlerFromValue(svc, methodVal reflect.Value, reqType reflect.Type) weedhttp.HandlerFunc {
	return func(c *weedhttp.Ctx) error {
		reqVal := reflect.New(reqType.Elem())
		reqPtr := reqVal.Interface()

		httpMethod := c.Request().Method
		if httpMethod == "POST" || httpMethod == "PUT" || httpMethod == "PATCH" {
			if c.Request().Body != nil {
				if err := json.NewDecoder(c.Request().Body).Decode(reqPtr); err != nil {
					return c.JSON(400, map[string]string{"error": "invalid request body"})
				}
			}
		}

		if err := bindRequestTags(c, reqVal); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request parameters: " + err.Error()})
		}

		ctxVal := reflect.ValueOf(c.Request().Context())
		results := methodVal.Call([]reflect.Value{svc, ctxVal, reqVal})

		return processResults(c, results)
	}
}

func processResults(c *weedhttp.Ctx, results []reflect.Value) error {
	errVal := results[1]
	if !errVal.IsNil() {
		err := errVal.Interface().(error)
		var ce ContractError
		if errors.As(err, &ce) {
			return c.JSON(ce.StatusCode(), map[string]string{"error": ce.Error()})
		}
		return c.JSON(500, map[string]string{"error": "internal server error"})
	}

	respVal := results[0]
	return c.JSON(200, respVal.Interface())
}

func getHttpMethod(dto any) string {
	var method = "GET"
	fields := getFields(dto)
	for _, item := range fields {
		if item.Tag.Get("form") != "" {
			return http.MethodPost
		}
		if item.Tag.Get("json") != "" {
			return http.MethodPost
		}
	}
	return method
}

// getFields get all the define of fields
func getFields(dto any) []reflect.StructField {
	var result []reflect.StructField
	t := reflect.TypeOf(dto)

	// 1. Check if it's a pointer and get the underlying element
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// 2. Ensure we are actually dealing with a struct now
	// (Prevents panic if someone passes an int or a slice)
	if t.Kind() != reflect.Struct {
		return nil
	}
	for i := 0; i < t.NumField(); i++ {
		result = append(result, t.Field(i))
	}
	return result
}

func toSnakeCase(str string) string {
	runes := []rune(str)
	var result []rune

	for i, r := range runes {
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Check if previous char was lowercase or if next char is lowercase
			// (to handle acronyms like "APIRequest" -> "api_request")
			prev := runes[i-1]
			if (prev >= 'a' && prev <= 'z') || (i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z') {
				result = append(result, '_')
			}
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}
