package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	weedhttp "github.com/wdvn/weed/internal/core/http"
)

// Example Request Struct that implements RouteDescriptor
type CreateUserReq struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func (r *CreateUserReq) Method() string { return "POST" }
func (r *CreateUserReq) Path() string   { return "/api/users" }

type CreateUserResp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GetUserReq struct {
	ID string `json:"id"` // Would normally bind from URL params or query
}

func (r *GetUserReq) Method() string { return "GET" }
func (r *GetUserReq) Path() string   { return "/api/users/:id" }

type GetUserResp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Example Service Implementation for Struct Registration
type UserService struct{}

func (s *UserService) CreateUser(ctx context.Context, req *CreateUserReq) (*CreateUserResp, error) {
	if req.Name == "" {
		return nil, NewError(400, "name is required")
	}
	return &CreateUserResp{
		ID:   "123",
		Name: req.Name,
	}, nil
}

func (s *UserService) GetUser(ctx context.Context, req *GetUserReq) (*GetUserResp, error) {
	return &GetUserResp{
		ID:   "123",
		Name: "Alice",
	}, nil
}

func (s *UserService) IgnoreMe(ctx context.Context, name string) error {
	return nil
}

// Example Interface for Interface Registration
type IUserService interface {
	CreateUser(ctx context.Context, req *CreateUserReq) (*CreateUserResp, error)
	GetUser(ctx context.Context, req *GetUserReq) (*GetUserResp, error)
}

// Struct that implements IUserService
type UserServiceImpl struct{}

func (s *UserServiceImpl) CreateUser(ctx context.Context, req *CreateUserReq) (*CreateUserResp, error) {
	if req.Name == "" {
		return nil, NewError(400, "name is required")
	}
	return &CreateUserResp{
		ID:   "456",
		Name: req.Name,
	}, nil
}

func (s *UserServiceImpl) GetUser(ctx context.Context, req *GetUserReq) (*GetUserResp, error) {
	return &GetUserResp{
		ID:   "456",
		Name: "Bob",
	}, nil
}

func TestRegister(t *testing.T) {
	router := weedhttp.NewRouter()
	svc := &UserService{}

	err := Register(router.RouterGroup, svc)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	t.Run("Valid POST request", func(t *testing.T) {
		reqBody := `{"name":"Bob","age":30}`
		req := httptest.NewRequest("POST", "/api/users", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var resp CreateUserResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp.ID != "123" || resp.Name != "Bob" {
			t.Errorf("Unexpected response body: %s", rr.Body.String())
		}
	})

	t.Run("Invalid POST request (Contract Error)", func(t *testing.T) {
		reqBody := `{"age":30}` // missing name
		req := httptest.NewRequest("POST", "/api/users", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rr.Code)
		}

		var resp map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp["error"] != "name is required" {
			t.Errorf("Unexpected error message: %s", resp["error"])
		}
	})

	t.Run("Valid GET request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users/123", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})
}

func TestRegisterInterface(t *testing.T) {
	router := weedhttp.NewRouter()

	// Create the implementation instance
	svcImpl := &UserServiceImpl{}

	// Register the interface definition IUserService with its implementation svcImpl
	err := RegisterInterface[IUserService](router.RouterGroup, svcImpl)
	if err != nil {
		t.Fatalf("Failed to register interface: %v", err)
	}

	t.Run("Valid POST request via Interface", func(t *testing.T) {
		reqBody := `{"name":"Charlie","age":25}`
		req := httptest.NewRequest("POST", "/api/users", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var resp CreateUserResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		// ID should be 456 from UserServiceImpl
		if resp.ID != "456" || resp.Name != "Charlie" {
			t.Errorf("Unexpected response body: %s", rr.Body.String())
		}
	})

	t.Run("Valid GET request via Interface", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users/456", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var resp GetUserResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		// Name should be Bob from UserServiceImpl
		if resp.ID != "456" || resp.Name != "Bob" {
			t.Errorf("Unexpected response body: %s", rr.Body.String())
		}
	})
}
