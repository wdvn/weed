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
		w.Write([]byte("users list"))
	})

	// Test GET route with parameter
	router.GET("/users/:id", func(w http.ResponseWriter, r *http.Request, params Params) {
		id := params.Get("id")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("user " + id))
	})

	// Test catch-all route
	router.GET("/files/*filepath", func(w http.ResponseWriter, r *http.Request, params Params) {
		filepath := params.Get("filepath")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("file " + filepath))
	})

	tests := []struct {
		method         string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{"GET", "/users", http.StatusOK, "users list"},
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
