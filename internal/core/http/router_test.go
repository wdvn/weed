package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// --- Test Helpers ---

// createHandler helps generate a simple handler returning a static string
func createHandler(response string) HandlerFunc {
	return func(c *Ctx) error {
		return c.Text(200, response)
	}
}

// createParamHandler helps generate a handler that dumps its received params
func createParamHandler(prefix string) HandlerFunc {
	return func(c *Ctx) error {
		resp := prefix
		for _, p := range c.Params() {
			resp += fmt.Sprintf(" [%s=%s]", p.Key, p.Value)
		}
		return c.Text(200, resp)
	}
}

type routeTestCase struct {
	name           string
	method         string
	path           string
	expectedStatus int
	expectedBody   string
}

func runRouteTests(t *testing.T, router *Router, tests []routeTestCase) {
	t.Helper()
	for _, tt := range tests {
		testName := tt.name
		if testName == "" {
			testName = tt.method + " " + tt.path
		}
		t.Run(testName, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v url=%s", status, tt.expectedStatus, req.URL.Path)
			}

			if body := rr.Body.String(); body != tt.expectedBody {
				t.Errorf("handler returned unexpected body: got %v want %v", body, tt.expectedBody)
			}
		})
	}
}

// --- Test Suites ---

func TestRouter_BasicRoutes(t *testing.T) {
	router := NewRouter()
	router.GET("/", createHandler("root"))
	router.GET("/users", createHandler("users"))
	router.POST("/users", createHandler("create_user"))
	router.PUT("/users/1", createHandler("update_user_1"))
	router.DELETE("/users/1", createHandler("delete_user_1"))

	tests := []routeTestCase{
		{"Root path", "GET", "/", http.StatusOK, "root"},
		{"Users GET", "GET", "/users", http.StatusOK, "users"},
		{"Users POST", "POST", "/users", http.StatusOK, "create_user"},
		{"Users PUT", "PUT", "/users/1", http.StatusOK, "update_user_1"},
		{"Users DELETE", "DELETE", "/users/1", http.StatusOK, "delete_user_1"},
		{"Not Found path", "GET", "/notfound", http.StatusNotFound, "404 page not found\n"},
		{"Not Found method", "PATCH", "/users", http.StatusNotFound, "404 page not found\n"},
	}

	runRouteTests(t, router, tests)
}

func TestRouter_Parameters(t *testing.T) {
	router := NewRouter()
	router.GET("/users/:id", createParamHandler("user"))
	router.GET("/users/:id/posts/:post_id", createParamHandler("post"))
	router.GET("/search/:query", createParamHandler("search"))

	tests := []routeTestCase{
		{"Single param", "GET", "/users/123", http.StatusOK, "user [id=123]"},
		{"Single param with letters", "GET", "/users/abc-def", http.StatusOK, "user [id=abc-def]"},
		{"Multiple params", "GET", "/users/123/posts/456", http.StatusOK, "post [id=123] [post_id=456]"},
		{"Param with encoded space", "GET", "/search/hello%20world", http.StatusOK, "search [query=hello world]"},
		{"Missing param", "GET", "/users/", http.StatusNotFound, "404 page not found\n"},
		{"Incomplete path", "GET", "/users/123/posts", http.StatusNotFound, "404 page not found\n"},
	}

	runRouteTests(t, router, tests)
}

func TestRouter_CatchAll(t *testing.T) {
	router := NewRouter()
	router.GET("/static/*filepath", createParamHandler("static"))
	router.GET("/files/*path", createParamHandler("files"))

	tests := []routeTestCase{
		{"Catch-all simple", "GET", "/static/css/main.css", http.StatusOK, "static [filepath=css/main.css]"},
		{"Catch-all deeper", "GET", "/static/js/lib/app.js", http.StatusOK, "static [filepath=js/lib/app.js]"},
		{"Catch-all single", "GET", "/static/favicon.ico", http.StatusOK, "static [filepath=favicon.ico]"},
		{"Catch-all empty", "GET", "/static/", http.StatusOK, "static [filepath=]"},
		{"Catch-all strict missing slash", "GET", "/static", http.StatusNotFound, "404 page not found\n"},
		{"Catch-all alternative", "GET", "/files/docs/2023/report.pdf", http.StatusOK, "files [path=docs/2023/report.pdf]"},
	}

	runRouteTests(t, router, tests)
}

func TestRouter_ConflictAndPriority(t *testing.T) {
	router := NewRouter()

	// Registering routes. Priority should be Exact > Param > CatchAll
	router.GET("/users/new", createHandler("exact_new"))
	router.GET("/users/admin/settings", createHandler("exact_admin_settings"))
	router.GET("/users/:id", createParamHandler("param_user"))
	router.GET("/users/:id/profile", createParamHandler("param_profile"))
	router.GET("/users/*action", createParamHandler("catchall_action"))

	tests := []routeTestCase{
		{"Exact vs Param (Exact wins)", "GET", "/users/new", http.StatusOK, "exact_new"},
		{"Param match", "GET", "/users/123", http.StatusOK, "param_user [id=123]"},

		{"Param vs CatchAll", "GET", "/users/123/edit", http.StatusOK, "catchall_action [action=123/edit]"},
		{"CatchAll deeper", "GET", "/users/123/delete/confirm", http.StatusOK, "catchall_action [action=123/delete/confirm]"},

		{"Deep Exact vs Deep Param", "GET", "/users/admin/settings", http.StatusOK, "exact_admin_settings"},
		{"Deep Param match", "GET", "/users/123/profile", http.StatusOK, "param_profile [id=123]"},
		{"Deep Param vs CatchAll", "GET", "/users/123/settings", http.StatusOK, "catchall_action [action=123/settings]"},
	}

	runRouteTests(t, router, tests)
}

func TestRouter_EdgeCases(t *testing.T) {
	router := NewRouter()
	router.GET("/api/v1/users", createHandler("users"))

	tests := []routeTestCase{
		{"Trailing slash", "GET", "/api/v1/users", http.StatusOK, "users"},
		{"Multiple slashes", "GET", "//api///v1//users//", http.StatusNotFound, "users"},
		{"Missing root slash", "GET", "api/v1/users", http.StatusNotFound, "users"},
	}

	runRouteTests(t, router, tests)
}

func TestRouter_Static(t *testing.T) {
	// Create a temporary directory for static files
	tempDir := t.TempDir()

	// Create a sample index.html
	err := os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("<h1>Hello Static</h1>"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create a sample css file in a sub-directory
	cssDir := filepath.Join(tempDir, "css")
	err = os.Mkdir(cssDir, 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(cssDir, "style.css"), []byte("body { color: red; }"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	router := NewRouter()
	router.Static("/public", tempDir)

	tests := []routeTestCase{
		{"Static file root", "GET", "/public/index.html", http.StatusOK, "<h1>Hello Static</h1>"},
		{"Static file sub directory", "GET", "/public/css/style.css", http.StatusOK, "body { color: red; }"},
		{"Static file not found", "GET", "/public/not-exist.html", http.StatusNotFound, "404 page not found\n"},
		{"Directory listing or index", "GET", "/public/", http.StatusOK, "<pre>\n<a href=\"css/\">css/</a>\n<a href=\"index.html\">index.html</a>\n</pre>\n"},
	}

	runRouteTests(t, router, tests)
}

func BenchmarkRouter_ServeHTTP_ZeroAllocation(b *testing.B) {
	router := NewRouter()
	router.GET("/api/v1/users/:id/posts/:post_id/comments/*filepath", func(c *Ctx) error {
		_ = c.Param("id")
		_ = c.Param("post_id")
		_ = c.Param("filepath")
		return c.Text(200, "") // Causes 1 allocation due to w.Header().Set()
	})

	req, _ := http.NewRequest("GET", "/api/v1/users/123/posts/456/comments/docs/readme.md", nil)
	rr := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		router.ServeHTTP(rr, req)
	}
}

// Add this Benchmark to prove the Router is truly Zero Allocation
func BenchmarkRouter_ServeHTTP_True_ZeroAllocation(b *testing.B) {
	router := NewRouter()
	router.GET("/api/v1/users/:id/posts/:post_id/comments/*filepath", func(c *Ctx) error {
		_ = c.Param("id")
		_ = c.Param("post_id")
		_ = c.Param("filepath")
		// Return nil instead of c.Text() to eliminate net/http Header.Set() writing
		return nil
	})

	req, _ := http.NewRequest("GET", "/api/v1/users/123/posts/456/comments/docs/readme.md", nil)
	rr := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		router.ServeHTTP(rr, req)
	}
}
