package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter(t *testing.T) {
	router := NewRouter()

	// Test GET route with simple path
	router.GET("/users", func(w http.ResponseWriter, r *http.Request, params Params) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("users list"))
	})

	// Test GET route with parameter
	router.GET("/users/:id", func(w http.ResponseWriter, r *http.Request, params Params) {
		id := params.Get("id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("user " + id))
	})

	// Test catch-all route
	router.GET("/files/*filepath", func(w http.ResponseWriter, r *http.Request, params Params) {
		filepath := params.Get("filepath")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("file " + filepath))
	})

	tests := []struct {
		method         string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{"GET", "/users", http.StatusOK, "users list"},
		{"GET", "/users/", http.StatusOK, "users list"}, // Trailing slash is handled
		{"GET", "/users/123", http.StatusOK, "user 123"},
		{"GET", "/files/docs/readme.txt", http.StatusOK, "file docs/readme.txt"},
		{"GET", "/notfound", http.StatusNotFound, "404 page not found\n"},
		{"POST", "/users", http.StatusNotFound, "404 page not found\n"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}

			if body := rr.Body.String(); body != tt.expectedBody {
				t.Errorf("handler returned unexpected body: got %v want %v",
					body, tt.expectedBody)
			}
		})
	}
}

func TestRouter_Conflict(t *testing.T) {
	router := NewRouter()

	// Đăng ký route cụ thể trước
	router.GET("/users/new", func(w http.ResponseWriter, r *http.Request, params Params) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("new user"))
	})

	// Đăng ký route với tham số sau
	router.GET("/users/:id", func(w http.ResponseWriter, r *http.Request, params Params) {
		id := params.Get("id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("user " + id))
	})

	// Đăng ký route catch-all
	router.GET("/users/*action", func(w http.ResponseWriter, r *http.Request, params Params) {
		action := params.Get("action")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("action " + action))
	})

	// Xung đột sâu hơn:
	router.GET("/users/:id/profile", func(w http.ResponseWriter, r *http.Request, params Params) {
		id := params.Get("id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("profile " + id))
	})

	router.GET("/users/admin/settings", func(w http.ResponseWriter, r *http.Request, params Params) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("admin settings"))
	})

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Exact match priority over param",
			path:           "/users/new",
			expectedStatus: http.StatusOK,
			expectedBody:   "new user",
		},
		{
			name:           "Param match",
			path:           "/users/123",
			expectedStatus: http.StatusOK,
			expectedBody:   "user 123",
		},
		{
			name:           "Catch-all match",
			path:           "/users/123/edit",
			expectedStatus: http.StatusOK,
			expectedBody:   "action 123/edit",
		},
		{
			name:           "Deep exact match over deep param match",
			path:           "/users/admin/settings",
			expectedStatus: http.StatusOK,
			expectedBody:   "admin settings",
		},
		{
			name:           "Deep param match",
			path:           "/users/123/profile",
			expectedStatus: http.StatusOK,
			expectedBody:   "profile 123",
		},
		{
			name:           "Deep param with no match should fallback correctly if catch-all exists (or 404)",
			path:           "/users/123/settings",
			expectedStatus: http.StatusOK,
			expectedBody:   "action 123/settings", // Khớp với route /users/*action
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}

			if body := rr.Body.String(); body != tt.expectedBody {
				t.Errorf("handler returned unexpected body: got %v want %v",
					body, tt.expectedBody)
			}
		})
	}
}

func BenchmarkRouter_ServeHTTP_ZeroAllocation(b *testing.B) {
	router := NewRouter()
	router.GET("/api/v1/users/:id/posts/:post_id/comments/*filepath", func(w http.ResponseWriter, r *http.Request, params Params) {
		// Mock handler logic
		_ = params.Get("id")
		_ = params.Get("post_id")
		_ = params.Get("filepath")
	})

	req, _ := http.NewRequest("GET", "/api/v1/users/123/posts/456/comments/docs/readme.md", nil)
	rr := httptest.NewRecorder()

	b.ReportAllocs() // Quan trọng để Go benchmark báo cáo lượng RAM cấp phát
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		router.ServeHTTP(rr, req)
	}
}
