# Weed Framework

Weed is a lightweight and powerful web framework for Go, designed for building RESTful APIs with a focus on contract-driven development and simplicity.

## Features

- **Simple and Fast**: Built on a high-performance router.
- **Contract-Driven Development**: Define your API using Go interfaces and let Weed handle the rest.
- **Automatic Request Binding**: Automatically bind path parameters, query strings, and headers to your request structs.
- **OpenAPI Generation**: Automatically generate OpenAPI 3.0 specifications from your code.
- **Middleware Support**: Easily add middleware to your routes.
- **Static File Serving**: Serve static files with a single line of code.

## Getting Started

### 1. Installation

```sh
go get github.com/wdvn/weed
```

### 2. Your First Application

Create a `main.go` file:

```go
package main

import "github.com/wdvn/weed"

func main() {
	app := weed.New()

	app.GET("/ping", func(c *weed.Ctx) error {
		return c.JSON(200, map[string]string{"message": "pong"})
	})

	app.Serve(":8080")
}
```

Run the application:

```sh
go run main.go
```

You can now visit `http://localhost:8080/ping` and you should see `{"message":"pong"}`.

## Contract-Driven Development

Weed excels at contract-driven development. You can define your services as Go interfaces and Weed will automatically expose them as HTTP endpoints.

### 1. Define Your Contract

Define request/response structs and a service interface.

```go
package main

import "context"

// MyRequest defines the request structure.
type MyRequest struct {
	ID   int    `path:"id"`
	Name string `query:"name"`
}

// MyResponse defines the response structure.
type MyResponse struct {
	Message string `json:"message"`
}

// ExampleService defines the contract for our service.
type ExampleService interface {
	GetExample(ctx context.Context, req *MyRequest) (*MyResponse, error)
}
```

### 2. Implement Your Service

Implement the service interface.

```go
import (
	"context"
	"fmt"
	"github.com/wdvn/weed/core/driven/rest"
)

// exampleServiceImpl implements the ExampleService interface.
type exampleServiceImpl struct{}

func (s *exampleServiceImpl) GetExample(ctx context.Context, req *MyRequest) (*MyResponse, error) {
	if req.ID == 0 {
		return nil, rest.NewError(400, "ID is required")
	}
	message := fmt.Sprintf("Hello, %s! Your ID is %d.", req.Name, req.ID)
	return &MyResponse{Message: message}, nil
}
```

### 3. Register Your Service

Create a `weed.App` and register your service implementation with `AddService`.

```go
package main

import "github.com/wdvn/weed"

func main() {
	app := weed.New()

	service := &exampleServiceImpl{}
	app.AddService("/api", service) // Mounts the service under the /api prefix

	// This will automatically register a GET endpoint at /api/get_example/:id

	app.Serve(":8080")
}
```
The `AddService` method uses reflection to mount the service. The path is generated from the service name, method name, and the request struct tags.

## Manual Type-Safe Handlers

If you don't want to use the full interface-based service registration, you can still get type safety on individual routes by using `rest.Handler`.

```go
package main

import (
    "context"
    "fmt"
    "github.com/wdvn/weed"
    "github.com/wdvn/weed/core/driven/rest"
)

// MyRequest and MyResponse are the same as in the previous example

// MyHandler implements the logic for our endpoint.
func MyHandler(ctx context.Context, req *MyRequest) (*MyResponse, error) {
	if req.ID == 0 {
		return nil, rest.NewError(400, "ID is required")
	}
	message := fmt.Sprintf("Hello, %s! Your ID is %d.", req.Name, req.ID)
	return &MyResponse{Message: message}, nil
}

func main() {
	app := weed.New()

	// Wrap and register the handler
	app.GET("/example/:id", rest.Handler(MyHandler))

	app.Serve(":8080")
}
```

## OpenAPI Generation

Weed can automatically generate an OpenAPI 3.0 specification for your registered services.

```go
package main

import "github.com/wdvn/weed"

func main() {
	app := weed.New()

	// ... register your services with app.AddService ...

	// Add an endpoint to serve the OpenAPI spec
	app.GET("/swagger.json", func(c *weed.Ctx) error {
		openapiSpec := app.GenerateOpenAPI()
		c.Response().Header().Set("Content-Type", "application/json")
		_, _ = c.Response().Write(openapiSpec)
		return nil
	})

	app.Serve(":8080")
}
```
You can now access the OpenAPI specification at `http://localhost:8080/swagger.json`.

## API Reference

### `weed.App`

The main application object.

- `New() *App`: Creates a new `App` instance.
- `Serve(port string) error`: Starts the HTTP server.
- `Use(middleware MiddlewareFunc)`: Adds middleware to the application.
- `GET(path string, handler HandlerFunc)`: Registers a GET route.
- `POST(path string, handler HandlerFunc)`: Registers a POST route.
- `PUT(path string, handler HandlerFunc)`: Registers a PUT route.
- `DELETE(path string, handler HandlerFunc)`: Registers a DELETE route.
- `Group(prefix string, ...MiddlewareFunc) *RouterGroup`: Creates a new router group.
- `Static(prefix, root string)`: Serves static files.
- `AddService(name string, sv any)`: Registers a service implementation. The `name` is used as a path prefix.
- `GenerateOpenAPI() []byte`: Generates the OpenAPI 3.0 JSON specification.

### Struct Tags

- `path:"<param_name>"`: Binds a path parameter to a struct field.
- `query:"<param_name>"`: Binds a query parameter to a struct field.
- `header:"<header_name>"`: Binds a header value to a struct field.
- `json:"<name>"`: Specifies the JSON field name for request/response bodies and OpenAPI generation.

Supported field types for binding are: `string`, `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`, and `bool`.
