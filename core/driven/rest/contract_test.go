package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	weedhttp "github.com/wdvn/weed/core/http"
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
	ID string `path:"id"` // Binding from URL parameter
}

func (r *GetUserReq) Method() string { return "GET" }
func (r *GetUserReq) Path() string   { return "/api/users/:id" }

type GetUserResp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SearchUserReq struct {
	Query string `query:"q"`
	Page  int    `query:"page"`
	Token string `header:"X-Token"`
}

func (r *SearchUserReq) Method() string { return "GET" }
func (r *SearchUserReq) Path() string   { return "/api/users" }

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
		ID:   req.ID,
		Name: "Alice",
	}, nil
}

func (s *UserService) SearchUser(ctx context.Context, req *SearchUserReq) (*GetUserResp, error) {
	if req.Token != "secret" {
		return nil, NewError(401, "unauthorized")
	}
	return &GetUserResp{
		ID:   "search-" + req.Query,
		Name: "Search Result",
	}, nil
}

func (s *UserService) IgnoreMe(ctx context.Context, name string) error {
	return nil
}

// Example Interface for Interface Registration
type IUserService interface {
	CreateUser(ctx context.Context, req *CreateUserReq) (*CreateUserResp, error)
	GetUser(ctx context.Context, req *GetUserReq) (*GetUserResp, error)
	SearchUser(ctx context.Context, req *SearchUserReq) (*GetUserResp, error)
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
		ID:   req.ID,
		Name: "Bob",
	}, nil
}

func (s *UserServiceImpl) SearchUser(ctx context.Context, req *SearchUserReq) (*GetUserResp, error) {
	if req.Token != "secret" {
		return nil, NewError(401, "unauthorized")
	}
	return &GetUserResp{
		ID:   "search-" + req.Query,
		Name: "Search Result",
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

	t.Run("Valid GET request with path param binding", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users/789", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var resp GetUserResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp.ID != "789" {
			t.Errorf("Path param not bound, got: %s", resp.ID)
		}
	})

	t.Run("Valid GET request with query and header binding", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users?q=test&page=1", nil)
		req.Header.Set("X-Token", "secret")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var resp GetUserResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp.ID != "search-test" {
			t.Errorf("Query param not bound, got: %s", resp.ID)
		}
	})

	t.Run("Invalid GET request with missing header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users?q=test", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rr.Code)
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

		if resp.ID != "456" || resp.Name != "Charlie" {
			t.Errorf("Unexpected response body: %s", rr.Body.String())
		}
	})

	t.Run("Valid GET request via Interface with path param binding", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users/999", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var resp GetUserResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp.ID != "999" || resp.Name != "Bob" {
			t.Errorf("Unexpected response body: %s", rr.Body.String())
		}
	})

	t.Run("Valid GET request via Interface with query and header binding", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users?q=hello", nil)
		req.Header.Set("X-Token", "secret")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var resp GetUserResp
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp.ID != "search-hello" {
			t.Errorf("Unexpected response body: %s", rr.Body.String())
		}
	})
}
