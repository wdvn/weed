package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	weedhttp "github.com/wdvn/weed/internal/core/http"
)

// RouteDescriptor is an interface that request structs can implement to define their route contract.
type RouteDescriptor interface {
	Method() string
	Path() string
}

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
					// It's possible the body is empty or not JSON, we can ignore EOF or handle explicitly
					// For strict contract, decoding error is a bad request
					return c.JSON(400, map[string]string{"error": "invalid request body: " + err.Error()})
				}
			}
		}

		// Execute the strongly-typed handler
		resp, err := h(c.Request().Context(), &req)
		if err != nil {
			if ce, ok := err.(ContractError); ok {
				return c.JSON(ce.StatusCode(), map[string]string{"error": ce.Error()})
			}
			return c.JSON(500, map[string]string{"error": "internal server error"})
		}

		// Return the response as JSON
		return c.JSON(200, resp)
	}
}

// Register iterates through all exported methods of the provided service implementation.
// It looks for methods with the signature: func (ctx context.Context, req *ReqType) (*RespType, error)
// where ReqType implements the RouteDescriptor interface. It then automatically registers
// these routes onto the provided RouterGroup.
//
// Deprecated: Consider using RegisterInterface for stronger contract-driven development.
func Register(router *weedhttp.RouterGroup, service any) error {
	svcType := reflect.TypeOf(service)
	svcVal := reflect.ValueOf(service)

	if svcType.Kind() != reflect.Ptr || svcVal.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("service must be a pointer to a struct")
	}

	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	errType := reflect.TypeOf((*error)(nil)).Elem()
	routeDescType := reflect.TypeOf((*RouteDescriptor)(nil)).Elem()

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

		// Request must implement RouteDescriptor
		if !reqType.Implements(routeDescType) {
			continue
		}

		// Check return types
		if !mType.Out(1).Implements(errType) {
			continue
		}

		// We have a match! Let's instantiate the request to get the method and path
		reqInstance := reflect.New(reqType.Elem()).Interface().(RouteDescriptor)
		httpMethod := reqInstance.Method()
		httpPath := reqInstance.Path()

		if httpMethod == "" || httpPath == "" {
			continue // Skip if the route is invalid
		}

		// Create a dynamic HandlerFunc that bridges the weedhttp.Ctx to the reflect method call
		handler := createDynamicHandler(svcVal, method.Func, reqType)

		// Register with the router
		router.Handle(httpMethod, httpPath, handler)
	}

	return nil
}

// RegisterInterface registers routes based on an interface definition.
// T must be an interface type. service must be an implementation of T.
// This allows true contract-driven design where the interface acts as the router contract.
func RegisterInterface[T any](router *weedhttp.RouterGroup, service T) error {
	// Get the interface type from the generic parameter T
	ifaceType := reflect.TypeOf((*T)(nil)).Elem()

	if ifaceType.Kind() != reflect.Interface {
		return fmt.Errorf("T must be an interface type")
	}

	svcVal := reflect.ValueOf(service)

	// Ensure the service actually implements the interface
	if !reflect.TypeOf(service).Implements(ifaceType) {
		return fmt.Errorf("service does not implement interface %s", ifaceType.Name())
	}

	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	errType := reflect.TypeOf((*error)(nil)).Elem()
	routeDescType := reflect.TypeOf((*RouteDescriptor)(nil)).Elem()

	for i := 0; i < ifaceType.NumMethod(); i++ {
		method := ifaceType.Method(i)
		mType := method.Type

		// For interface methods, NumIn() does not include the receiver, so it expects 2 args
		if mType.NumIn() != 2 || mType.NumOut() != 2 {
			continue
		}

		// Check parameter types
		if !mType.In(0).Implements(ctxType) {
			continue
		}

		reqType := mType.In(1)
		if reqType.Kind() != reflect.Ptr {
			continue
		}

		// Request must implement RouteDescriptor
		if !reqType.Implements(routeDescType) {
			continue
		}

		// Check return types
		if !mType.Out(1).Implements(errType) {
			continue
		}

		// Instantiate the request to get the HTTP Method and Path
		reqInstance := reflect.New(reqType.Elem()).Interface().(RouteDescriptor)
		httpMethod := reqInstance.Method()
		httpPath := reqInstance.Path()

		if httpMethod == "" || httpPath == "" {
			continue
		}

		// Find the actual method implementation on the service struct
		implMethodVal := svcVal.MethodByName(method.Name)
		if !implMethodVal.IsValid() {
			return fmt.Errorf("method %s not found on service implementation", method.Name)
		}

		// Create the handler using the actual implementation method value
		handler := createDynamicHandlerFromValue(implMethodVal, reqType)

		router.Handle(httpMethod, httpPath, handler)
	}

	return nil
}

// createDynamicHandler creates a weedhttp.HandlerFunc that invokes the target method via reflection (for Structs)
func createDynamicHandler(svcVal reflect.Value, methodFunc reflect.Value, reqType reflect.Type) weedhttp.HandlerFunc {
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

		ctxVal := reflect.ValueOf(c.Request().Context())
		results := methodFunc.Call([]reflect.Value{svcVal, ctxVal, reqVal})

		return processResults(c, results)
	}
}

// createDynamicHandlerFromValue creates a weedhttp.HandlerFunc from an already bound method value (for Interfaces)
func createDynamicHandlerFromValue(methodVal reflect.Value, reqType reflect.Type) weedhttp.HandlerFunc {
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

		ctxVal := reflect.ValueOf(c.Request().Context())
		results := methodVal.Call([]reflect.Value{ctxVal, reqVal})

		return processResults(c, results)
	}
}

func processResults(c *weedhttp.Ctx, results []reflect.Value) error {
	errVal := results[1]
	if !errVal.IsNil() {
		err := errVal.Interface().(error)
		if ce, ok := err.(ContractError); ok {
			return c.JSON(ce.StatusCode(), map[string]string{"error": ce.Error()})
		}
		return c.JSON(500, map[string]string{"error": "internal server error"})
	}

	respVal := results[0]
	return c.JSON(200, respVal.Interface())
}
