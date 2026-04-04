# 🌿 Weed Framework

Weed is a lightweight, high-performance web framework for Go — zero dependency, zero allocation routing, and contract-driven API development.

## Features

- **Zero Allocation Router**: Radix tree (trie) based routing with zero heap allocation at runtime via `sync.Pool`
- **Contract-Driven Development**: Define your API using Go structs and let Weed handle routing, binding, and OpenAPI generation
- **Automatic Request Binding**: Bind path parameters, query strings, headers, and JSON bodies via struct tags
- **OpenAPI 3.0 Generation**: Auto-generate OpenAPI specs from registered routes with built-in Swagger UI
- **Global Route Metadata**: All routes are tracked in a radix tree based global registry (`core/meta`) for OpenAPI and introspection
- **Middleware Support**: Global, group-level, and per-route middleware
- **Static File Serving**: Serve static files with a single line of code
- **Generic Radix Tree**: Reusable `core/ds/radix` package with Go generics

## Getting Started

### Installation

```sh
go get github.com/wdvn/weed
```

### Hello World

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

```sh
$ curl http://localhost:8080/ping
{"message":"pong"}
```

---

## Routing

### Basic Routes

```go
app := weed.New()

app.GET("/users", func(c *weed.Ctx) error {
    return c.JSON(200, []string{"alice", "bob"})
})

app.POST("/users", func(c *weed.Ctx) error {
    return c.Text(201, "created")
})

app.PUT("/users/:id", func(c *weed.Ctx) error {
    id := c.Param("id")
    return c.JSON(200, map[string]string{"updated": id})
})

app.DELETE("/users/:id", func(c *weed.Ctx) error {
    return c.NoContent(204)
})
```

### Path Parameters

Use `:param` for single-segment parameters and `*param` for catch-all:

```go
// Single param: /users/123 → c.Param("id") = "123"
app.GET("/users/:id", func(c *weed.Ctx) error {
    return c.Text(200, "User: "+c.Param("id"))
})

// Multiple params: /users/123/posts/456
app.GET("/users/:id/posts/:postId", func(c *weed.Ctx) error {
    return c.JSON(200, map[string]string{
        "user": c.Param("id"),
        "post": c.Param("postId"),
    })
})

// Catch-all: /files/docs/2024/report.pdf → c.Param("filepath") = "docs/2024/report.pdf"
app.GET("/files/*filepath", func(c *weed.Ctx) error {
    return c.Text(200, "File: "+c.Param("filepath"))
})
```

Route matching priority: **Exact > Param (`:id`) > Catch-all (`*filepath`)**

---

## Router Groups

Groups share a common prefix and middleware:

```go
app := weed.New()

// Group with prefix
api := app.Group("/api")

// Nested group with middleware
v1 := api.Group("/v1", authMiddleware)

v1.GET("/users", listUsers)     // → GET /api/v1/users
v1.POST("/users", createUser)   // → POST /api/v1/users

// Further nesting
admin := v1.Group("/admin", adminOnlyMiddleware)
admin.GET("/stats", getStats)   // → GET /api/v1/admin/stats
```

---

## Middleware

Middleware wraps handlers in an onion-style execution chain.

### Global Middleware

```go
app := weed.New()
app.Use(middleware.Logger())
app.Use(middleware.Recover())
```

### Group Middleware

```go
api := app.Group("/api", middleware.Logger(), middleware.Secure())
```

### Per-Service Middleware

```go
app.AddService("users", &UserService{}, authMiddleware)
app.AddServiceToGroup(apiGroup, "admin", &AdminService{}, authMiddleware, adminMiddleware)
```

### Built-in Middleware

| Middleware | Description |
|---|---|
| `middleware.Logger()` | Logs method, path, status, and duration |
| `middleware.Recover()` | Recovers from panics, returns HTTP 500 |
| `middleware.Secure()` | Sets security headers (XSS, CSRF, etc.) |
| `middleware.CSRF()` | CSRF protection via cookie + header validation |

### Custom Middleware

```go
func RateLimiter() weed.MiddlewareFunc {
    return func(next weed.HandlerFunc) weed.HandlerFunc {
        return func(c *weed.Ctx) error {
            // Before handler
            if isLimited(c.Request().RemoteAddr) {
                return c.JSON(429, map[string]string{"error": "too many requests"})
            }
            
            err := next(c) // Call the next handler
            
            // After handler
            logRequest(c)
            return err
        }
    }
}
```

---

## Contract-Driven Development

Define APIs as Go structs with automatic routing, binding, and OpenAPI generation.

### 1. Define Request/Response Structs

```go
// Request struct — tags determine binding source and HTTP method
type GetUserRequest struct {
    ID   int    `path:"id"`       // bound from URL path
    Lang string `query:"lang"`    // bound from query string
}

// If request has `json` or `form` tags → auto-detected as POST
type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

type UserResponse struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}
```

### 2. Implement the Service

```go
type UserService struct{}

func (s *UserService) GetUser(ctx context.Context, req *GetUserRequest) (*UserResponse, error) {
    if req.ID == 0 {
        return nil, rest.NewError(400, "ID is required")
    }
    return &UserResponse{ID: req.ID, Name: "Alice", Email: "alice@example.com"}, nil
}

func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*UserResponse, error) {
    // Auto-detected as POST because CreateUserRequest has `json` tags
    return &UserResponse{ID: 1, Name: req.Name, Email: req.Email}, nil
}
```

### 3. Mount the Service

```go
app := weed.New()

// Mounts at /users with auto-generated routes:
//   GET  /users/get_user/:id    → GetUser
//   POST /users/create_user     → CreateUser
app.AddService("users", &UserService{})

// Mount into a group with middleware
api := app.Group("/api/v1")
app.AddServiceToGroup(api, "users", &UserService{}, authMiddleware)
// Routes: GET /api/v1/users/get_user/:id, POST /api/v1/users/create_user

app.Serve(":8080")
```

### Method Name → Route Path Mapping

Method names are converted to `snake_case` and prefixed with the service name:

| Method Name | Route Path |
|---|---|
| `GetUser` | `GET /<name>/get_user` |
| `CreateUser` | `POST /<name>/create_user` |
| `UpdateProfile` | `POST /<name>/update_profile` |
| `ListAllItems` | `GET /<name>/list_all_items` |

### HTTP Method Detection

The HTTP method is auto-detected from the request struct:
- Has `json` or `form` tags → **POST**
- No body tags (only `path`, `query`, `header`) → **GET**

### Error Handling

Return `rest.ContractError` for custom HTTP status codes:

```go
func (s *UserService) GetUser(ctx context.Context, req *GetUserRequest) (*UserResponse, error) {
    user, err := db.FindUser(req.ID)
    if err != nil {
        return nil, rest.NewError(404, "user not found")
    }
    return user, nil
}
// Returns: HTTP 404 {"error": "user not found"}
```

Returning a plain `error` results in HTTP 500 `{"error": "internal server error"}`.

---

## Manual Type-Safe Handlers

Use `rest.Handler` for individual type-safe routes without full service registration:

```go
import "github.com/wdvn/weed/core/driven/rest"

type SearchRequest struct {
    Query string `query:"q"`
    Page  int    `query:"page"`
}

type SearchResponse struct {
    Results []string `json:"results"`
    Total   int      `json:"total"`
}

func SearchHandler(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
    return &SearchResponse{
        Results: []string{"result1", "result2"},
        Total:   2,
    }, nil
}

app.GET("/search", rest.Handler(SearchHandler))
// GET /search?q=hello&page=1
```

---

## Static File Serving

```go
app := weed.New()

// Serve files from ./public at /static/*
app.Static("/static", "./public")
// GET /static/css/style.css → ./public/css/style.css
// GET /static/js/app.js     → ./public/js/app.js

// Static files within a group
api := app.Group("/app")
api.Static("/assets", "./dist")
// GET /app/assets/main.js → ./dist/main.js
```

---

## OpenAPI & Swagger UI

### Auto-generate OpenAPI Spec

```go
app := weed.New()
app.AddService("users", &UserService{})
app.AddService("posts", &PostService{})

// Generate and serve Swagger UI
spec := app.GenerateOpenAPI()
app.Swagger("/docs", spec)

app.Serve(":8080")
// Swagger UI: http://localhost:8080/docs
// OpenAPI JSON: http://localhost:8080/docs/openapi.json
```

### How It Works

All routes are automatically tracked in a global metadata registry (`core/meta`). The `GenerateOpenAPI()` method reads this registry and generates the spec:

| Route Type | OpenAPI Response Schema |
|---|---|
| **Contract service** (`AddService`) | Full typed JSON schema from response struct |
| **Plain handler** (`app.GET(...)`) | Arbitrary JSON (`schema: {}`) |
| **Static files** (`app.Static(...)`) | Binary file download (`application/octet-stream`) |

### Route Metadata

Access registered route metadata programmatically:

```go
metas := app.RoutesMeta()
for _, m := range metas {
    fmt.Printf("%s %s (tag: %s)\n", m.Method, m.Path, m.Tag)
}
// Output:
// GET /users/get_user (tag: users)
// POST /users/create_user (tag: users)
// GET /static/*filepath (tag: )
```

---

## Context API

The `weed.Ctx` object provides access to request data and response writing:

### Request

```go
app.POST("/example/:id", func(c *weed.Ctx) error {
    // Path parameter
    id := c.Param("id")

    // Query parameter: /example/1?search=hello
    search := c.Query("search")

    // Read raw body
    body, err := c.Body()

    // Bind JSON body to struct
    var req MyStruct
    err = c.Bind(&req)

    // Form value
    name := c.FormValue("name")

    // Access raw request
    r := c.Request()
    
    // Request-scoped key-value store
    c.Set("user", currentUser)
    user := c.Get("user")

    return c.JSON(200, map[string]string{"id": id})
})
```

### Response

```go
// JSON response
c.JSON(200, map[string]string{"key": "value"})

// Plain text
c.Text(200, "hello world")

// HTML
c.HTML(200, "<h1>Hello</h1>")

// File download
c.File("./uploads/report.pdf")

// Raw bytes
c.Bytes(200, "image/png", imageBytes)

// Redirect
c.Redirect(302, "/login")

// No content
c.NoContent(204)

// Template rendering (requires SetRenderer)
c.Render(200, "index.html", templateData)
```

---

## Struct Tags Reference

| Tag | Binding Source | Example | Description |
|---|---|---|---|
| `path:"name"` | URL path | `path:"id"` | Binds `:id` from path |
| `query:"name"` | Query string | `query:"page"` | Binds `?page=1` |
| `header:"name"` | HTTP header | `header:"X-Token"` | Binds request header |
| `json:"name"` | JSON body | `json:"email"` | JSON field + triggers POST |
| `form:"name"` | Form body | `form:"file"` | Form field + triggers POST |

**Supported types**: `string`, `int`, `int8`–`int64`, `uint`, `uint8`–`uint64`, `float32`, `float64`, `bool`

---

## Architecture

```
github.com/wdvn/weed
├── weed.go              # App, routing shortcuts, AddService
├── swagger.go           # OpenAPI 3.0 generation + Swagger UI
├── middleware/           # Built-in middleware (Logger, Recover, Secure, CSRF)
└── core/
    ├── ds/radix/        # Generic radix tree (Tree[T]) — used by router & meta
    ├── http/            # Router, Context, Middleware, Renderer
    ├── meta/            # Global route metadata registry (radix tree backed)
    └── driven/rest/     # Contract system (Mount, Handler, request binding)
```

### Generic Radix Tree — `core/ds/radix`

The shared radix tree implementation powers both the HTTP router and the metadata registry:

```go
import "github.com/wdvn/weed/core/ds/radix"

// Create a tree that stores strings
tree := radix.New[string]()

// Insert values
tree.Insert("/users/:id", "user_handler")
tree.Insert("/posts/:id/comments", "comments_handler")

// Search with parameter extraction
var params radix.Params
value, found := tree.Search("/users/123", &params)
// value = "user_handler", params = [{Key:"id", Value:"123"}]

// Upsert with merge function
tree.Upsert("/users/:id", "updated_handler", func(existing *string, incoming string) {
    *existing = incoming
})

// Collect all values (DFS traversal)
all := tree.Collect()
// ["user_handler", "comments_handler"]
```

---

## Full Example

```go
package main

import (
    "context"
    "github.com/wdvn/weed"
    "github.com/wdvn/weed/core/driven/rest"
    "github.com/wdvn/weed/middleware"
)

// --- Contracts ---

type CreateTodoRequest struct {
    Title string `json:"title"`
}

type GetTodoRequest struct {
    ID int `path:"id"`
}

type TodoResponse struct {
    ID    int    `json:"id"`
    Title string `json:"title"`
    Done  bool   `json:"done"`
}

// --- Service ---

type TodoService struct{}

func (s *TodoService) CreateTodo(ctx context.Context, req *CreateTodoRequest) (*TodoResponse, error) {
    return &TodoResponse{ID: 1, Title: req.Title, Done: false}, nil
}

func (s *TodoService) GetTodo(ctx context.Context, req *GetTodoRequest) (*TodoResponse, error) {
    if req.ID == 0 {
        return nil, rest.NewError(400, "invalid ID")
    }
    return &TodoResponse{ID: req.ID, Title: "Buy milk", Done: false}, nil
}

// --- Main ---

func main() {
    app := weed.New()
    
    // Global middleware
    app.Use(middleware.Logger())
    app.Use(middleware.Recover())

    // Health check (plain handler)
    app.GET("/health", func(c *weed.Ctx) error {
        return c.JSON(200, map[string]string{"status": "ok"})
    })

    // Contract-driven API
    api := app.Group("/api/v1")
    app.AddServiceToGroup(api, "todos", &TodoService{})
    // Routes:
    //   POST /api/v1/todos/create_todo
    //   GET  /api/v1/todos/get_todo/:id

    // Static files
    app.Static("/public", "./static")

    // Swagger UI
    spec := app.GenerateOpenAPI()
    app.Swagger("/docs", spec)

    app.Serve(":8080")
}
```

## License

MIT License
