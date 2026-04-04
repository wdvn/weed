package radix

import (
	"fmt"
	"sort"
	"testing"
)

// =============================================================================
// SplitPath Tests
// =============================================================================
// SplitPath is the foundation of the tree — it converts a URL path string into
// a slice of segments that the tree uses to navigate its nodes.
// Getting this wrong means all Insert/Search operations will be incorrect.

func TestSplitPath_Basic(t *testing.T) {
	// A simple multi-segment path should produce one element per segment.
	// Leading and trailing slashes are ignored (empty segments are dropped).
	got := SplitPath("/api/v1/users")
	expect := []string{"api", "v1", "users"}
	assertSliceEqual(t, expect, got)
}

func TestSplitPath_Params(t *testing.T) {
	// Param segments like :id are kept as-is — they become wildcard nodes
	// in the tree. The ":" prefix is how the tree recognises them.
	got := SplitPath("/users/:id/posts/:postId")
	expect := []string{"users", ":id", "posts", ":postId"}
	assertSliceEqual(t, expect, got)
}

func TestSplitPath_CatchAll(t *testing.T) {
	// A catch-all segment (*filepath) captures everything after it,
	// so SplitPath must STOP splitting after the catch-all segment.
	// Anything after the * is irrelevant and should not appear in parts.
	got := SplitPath("/static/*filepath")
	expect := []string{"static", "*filepath"}
	assertSliceEqual(t, expect, got)
}

func TestSplitPath_RootPath(t *testing.T) {
	// The root path "/" has no segments at all — it maps to the tree root node.
	got := SplitPath("/")
	if len(got) != 0 {
		t.Errorf("expected empty slice for root path, got %v", got)
	}
}

func TestSplitPath_MultipleSlashes(t *testing.T) {
	// Consecutive slashes (e.g. "//api///v1//") should be treated the same
	// as "/api/v1" — empty segments are simply dropped.
	got := SplitPath("//api///v1//")
	expect := []string{"api", "v1"}
	assertSliceEqual(t, expect, got)
}

// =============================================================================
// Insert + Search Tests
// =============================================================================
// These tests verify the core read/write operations of the tree.
// We use Tree[string] for simplicity — the value is just a label.

func TestTree_InsertAndSearchExact(t *testing.T) {
	// Inserting exact (static) paths and searching for them.
	// This is the most basic operation: no wildcards, no params.
	tree := New[string]()
	tree.Insert("/users", "list_users")
	tree.Insert("/users/new", "new_user")
	tree.Insert("/posts", "list_posts")

	tests := []struct {
		path   string
		expect string
		found  bool
	}{
		{"/users", "list_users", true},
		{"/users/new", "new_user", true},
		{"/posts", "list_posts", true},
		{"/unknown", "", false},        // path doesn't exist in tree
		{"/users/old", "", false},      // only /users/new exists, not /users/old
		{"/users/new/edit", "", false}, // no deeper path registered
	}

	for _, tt := range tests {
		params := make(Params, 0, 10)
		val, found := tree.Search(tt.path, &params)
		if found != tt.found {
			t.Errorf("Search(%q): found=%v, want %v", tt.path, found, tt.found)
		}
		if found && *val != tt.expect {
			t.Errorf("Search(%q): value=%q, want %q", tt.path, *val, tt.expect)
		}
	}
}

func TestTree_SearchWithParams(t *testing.T) {
	// Param segments (:id) match any single path segment and extract its value.
	// The extracted value is stored in the Params slice for the caller to read.
	tree := New[string]()
	tree.Insert("/users/:id", "get_user")
	tree.Insert("/users/:id/posts/:postId", "get_post")

	// Test 1: Single param extraction
	// Searching /users/42 should match /users/:id and capture id=42
	params := make(Params, 0, 10)
	val, found := tree.Search("/users/42", &params)
	if !found || *val != "get_user" {
		t.Fatalf("expected get_user, got found=%v val=%v", found, val)
	}
	if params.Get("id") != "42" {
		t.Errorf("expected id=42, got id=%s", params.Get("id"))
	}

	// Test 2: Multiple param extraction
	// Searching /users/42/posts/7 should capture both id=42 and postId=7
	params = params[:0] // reset but keep capacity
	val, found = tree.Search("/users/42/posts/7", &params)
	if !found || *val != "get_post" {
		t.Fatalf("expected get_post, got found=%v val=%v", found, val)
	}
	if params.Get("id") != "42" || params.Get("postId") != "7" {
		t.Errorf("expected id=42,postId=7, got params=%v", params)
	}
}

func TestTree_SearchCatchAll(t *testing.T) {
	// Catch-all (*filepath) matches everything remaining in the path.
	// Unlike :param which matches exactly one segment, *catch-all captures
	// the entire rest of the URL including nested slashes.
	tree := New[string]()
	tree.Insert("/static/*filepath", "serve_file")

	tests := []struct {
		path     string
		filepath string
	}{
		{"/static/css/main.css", "css/main.css"},   // nested path
		{"/static/js/lib/app.js", "js/lib/app.js"}, // deeper nesting
		{"/static/favicon.ico", "favicon.ico"},     // single file
	}

	for _, tt := range tests {
		params := make(Params, 0, 10)
		val, found := tree.Search(tt.path, &params)
		if !found || *val != "serve_file" {
			t.Errorf("Search(%q): expected serve_file, got found=%v", tt.path, found)
			continue
		}
		if params.Get("filepath") != tt.filepath {
			t.Errorf("Search(%q): filepath=%q, want %q", tt.path, params.Get("filepath"), tt.filepath)
		}
	}
}

func TestTree_SearchPriority(t *testing.T) {
	// When multiple node types could match a segment, the tree uses
	// a strict priority order: Exact > Param > Catch-all
	//
	// Example tree:
	//   /users/new       — exact
	//   /users/:id       — param
	//   /users/*action   — catch-all
	//
	// Request for /users/new  → should match the exact "new" node, NOT :id
	// Request for /users/123  → should match :id
	// Request for /users/123/edit → no /users/:id/edit exists, falls to *action
	tree := New[string]()
	tree.Insert("/users/new", "exact_new")
	tree.Insert("/users/:id", "param_user")
	tree.Insert("/users/*action", "catchall")

	tests := []struct {
		path   string
		expect string
		param  string // expected param value (key depends on route)
	}{
		// "new" matches the exact node, not :id
		{"/users/new", "exact_new", ""},
		// "123" has no exact match, falls to :id param
		{"/users/123", "param_user", "123"},
		// "123/edit" has no match at :id/edit depth, falls to *action
		{"/users/123/edit", "catchall", "123/edit"},
	}

	for _, tt := range tests {
		params := make(Params, 0, 10)
		val, found := tree.Search(tt.path, &params)
		if !found {
			t.Errorf("Search(%q): not found, expected %q", tt.path, tt.expect)
			continue
		}
		if *val != tt.expect {
			t.Errorf("Search(%q): got %q, want %q", tt.path, *val, tt.expect)
		}
	}
}

func TestTree_SearchParamBacktracking(t *testing.T) {
	// Backtracking test: when a param branch matches a segment but the
	// deeper path has no result, the tree must "undo" the param capture
	// and try the next wildcard branch (catch-all).
	//
	// Tree:
	//   /users/:id/profile  — only matches if next segment is "profile"
	//   /users/*action      — catch-all fallback
	//
	// Request: /users/123/settings
	// → tries :id=123, then looks for /settings under :id → not found
	// → backtracks, undoes :id capture
	// → tries *action=123/settings → found
	tree := New[string]()
	tree.Insert("/users/:id/profile", "user_profile")
	tree.Insert("/users/*action", "catchall")

	params := make(Params, 0, 10)
	val, found := tree.Search("/users/123/settings", &params)
	if !found || *val != "catchall" {
		t.Fatalf("expected catchall, got found=%v val=%v", found, val)
	}
	// After backtracking, only the catch-all param should remain
	if params.Get("action") != "123/settings" {
		t.Errorf("expected action=123/settings, got %v", params)
	}
	// The :id param should NOT leak — it was backtracked
	if params.Get("id") != "" {
		t.Errorf("id param should be empty after backtrack, got %q", params.Get("id"))
	}
}

// =============================================================================
// Insert Overwrite Tests
// =============================================================================

func TestTree_InsertOverwrite(t *testing.T) {
	// Insert on the same path twice should overwrite the value.
	// This is the expected behaviour for the HTTP router: the last
	// handler registered for a path wins.
	tree := New[string]()
	tree.Insert("/users", "v1")
	tree.Insert("/users", "v2")

	params := make(Params, 0, 10)
	val, found := tree.Search("/users", &params)
	if !found || *val != "v2" {
		t.Errorf("expected v2 after overwrite, got %v", val)
	}
}

// =============================================================================
// Upsert Tests
// =============================================================================
// Upsert is used by the metadata registry (core/meta) to merge route info
// from different sources. Handle() registers basic {method, path} first,
// then Mount() enriches it with {ReqType, RespType, Tag} via the merge func.

func TestTree_UpsertNewEntry(t *testing.T) {
	// When the path doesn't exist yet, Upsert should simply insert the value
	// (the merge function is not called).
	tree := New[string]()

	mergeCalled := false
	tree.Upsert("/users", "initial", func(existing *string, incoming string) {
		mergeCalled = true
	})

	if mergeCalled {
		t.Error("merge should NOT be called for a new entry")
	}

	params := make(Params, 0, 10)
	val, found := tree.Search("/users", &params)
	if !found || *val != "initial" {
		t.Errorf("expected initial, got %v", val)
	}
}

func TestTree_UpsertMergesExisting(t *testing.T) {
	// When the path already exists, Upsert calls the merge function
	// so the caller can decide how to combine old and new values.
	//
	// Real-world example: Handle() registers RouteMeta{Method:"GET", Path:"/users"}
	// Then Mount() upserts with RouteMeta{..., ReqType:..., RespType:..., Tag:"users"}
	// The merge function copies non-zero fields from incoming to existing.
	type info struct {
		Label string
		Count int
	}

	tree := New[info]()
	tree.Upsert("/users", info{Label: "basic", Count: 0}, func(existing *info, incoming info) {
		t.Error("should not merge on first insert")
	})

	// Second upsert: merge should be called, enriching Count while keeping Label
	tree.Upsert("/users", info{Label: "", Count: 42}, func(existing *info, incoming info) {
		if incoming.Count != 0 {
			existing.Count = incoming.Count
		}
		// Label is empty in incoming, so we don't overwrite
	})

	params := make(Params, 0, 10)
	val, found := tree.Search("/users", &params)
	if !found {
		t.Fatal("expected to find /users")
	}
	if val.Label != "basic" {
		t.Errorf("Label should be preserved as 'basic', got %q", val.Label)
	}
	if val.Count != 42 {
		t.Errorf("Count should be merged to 42, got %d", val.Count)
	}
}

func TestTree_UpsertWithParams(t *testing.T) {
	// Upsert should work correctly on parameterised paths.
	// /users/:id is a single node path, and two upserts should target
	// the exact same tree node.
	tree := New[int]()
	tree.Upsert("/users/:id", 1, func(existing *int, incoming int) {
		*existing += incoming
	})
	tree.Upsert("/users/:id", 10, func(existing *int, incoming int) {
		*existing += incoming
	})

	params := make(Params, 0, 10)
	val, found := tree.Search("/users/42", &params)
	if !found {
		t.Fatal("expected to find /users/:id")
	}
	// 1 (initial) + 10 (merged) = 11
	if *val != 11 {
		t.Errorf("expected 11 after upsert merge, got %d", *val)
	}
}

// =============================================================================
// Collect Tests
// =============================================================================
// Collect performs a DFS traversal and returns all stored values.
// This is used by meta.All() to return every registered RouteMeta.

func TestTree_CollectEmpty(t *testing.T) {
	// An empty tree should return an empty slice, not nil.
	tree := New[string]()
	result := tree.Collect()
	if len(result) != 0 {
		t.Errorf("expected empty collect, got %v", result)
	}
}

func TestTree_CollectAll(t *testing.T) {
	// Collect should return every value stored in the tree, regardless
	// of path depth or wildcard type. Order is DFS (depth-first).
	tree := New[string]()
	tree.Insert("/", "root")
	tree.Insert("/users", "users")
	tree.Insert("/users/:id", "user_by_id")
	tree.Insert("/posts", "posts")
	tree.Insert("/static/*filepath", "static")

	result := tree.Collect()

	// Verify all 5 values are present (order depends on DFS traversal)
	if len(result) != 5 {
		t.Fatalf("expected 5 values, got %d: %v", len(result), result)
	}

	// Sort for deterministic comparison
	sort.Strings(result)
	expect := []string{"posts", "root", "static", "user_by_id", "users"}
	assertSliceEqual(t, expect, result)
}

func TestTree_CollectOnlyEndpoints(t *testing.T) {
	// Collect should only return values at nodes that are endpoints.
	// Intermediate nodes (e.g. "api" in /api/v1/users) have no value
	// and should NOT appear in the result.
	tree := New[string]()
	tree.Insert("/api/v1/users", "users_endpoint")
	// "api" and "v1" nodes exist but have no value

	result := tree.Collect()
	if len(result) != 1 || result[0] != "users_endpoint" {
		t.Errorf("expected only [users_endpoint], got %v", result)
	}
}

// =============================================================================
// Generic Type Tests
// =============================================================================
// The tree is generic — it should work with any type T, not just strings.

func TestTree_GenericWithStruct(t *testing.T) {
	// Simulate the real-world use case: storing route metadata structs.
	type RouteMeta struct {
		Method string
		Path   string
		Tag    string
	}

	tree := New[RouteMeta]()
	tree.Insert("/users", RouteMeta{Method: "GET", Path: "/users", Tag: "users"})
	tree.Insert("/users/:id", RouteMeta{Method: "GET", Path: "/users/:id", Tag: "users"})

	params := make(Params, 0, 10)
	val, found := tree.Search("/users/123", &params)
	if !found {
		t.Fatal("expected to find /users/:id")
	}
	if val.Method != "GET" || val.Tag != "users" {
		t.Errorf("unexpected meta: %+v", val)
	}
}

func TestTree_GenericWithFunc(t *testing.T) {
	// Simulate the router use case: storing handler functions.
	// This proves the tree works with function types, not just data.
	type HandlerFunc func() string

	tree := New[HandlerFunc]()
	tree.Insert("/ping", func() string { return "pong" })

	params := make(Params, 0, 10)
	val, found := tree.Search("/ping", &params)
	if !found {
		t.Fatal("expected to find /ping")
	}
	result := (*val)()
	if result != "pong" {
		t.Errorf("expected pong, got %s", result)
	}
}

// =============================================================================
// Params Tests
// =============================================================================

func TestParams_Get(t *testing.T) {
	// Get returns the value for a key, or "" if the key doesn't exist.
	params := Params{
		{Key: "id", Value: "42"},
		{Key: "name", Value: "alice"},
	}

	if params.Get("id") != "42" {
		t.Error("expected id=42")
	}
	if params.Get("name") != "alice" {
		t.Error("expected name=alice")
	}
	if params.Get("missing") != "" {
		t.Error("expected empty string for missing key")
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestTree_RootPath(t *testing.T) {
	// The root path "/" maps to the tree's root node.
	// It should be insertable and searchable like any other path.
	tree := New[string]()
	tree.Insert("/", "home")

	params := make(Params, 0, 10)
	val, found := tree.Search("/", &params)
	if !found || *val != "home" {
		t.Errorf("expected home at root, got found=%v val=%v", found, val)
	}
}

func TestTree_SearchNotFound(t *testing.T) {
	// Searching for a path that doesn't exist should return nil, false.
	// The params slice should remain unchanged.
	tree := New[string]()
	tree.Insert("/users", "users")

	params := make(Params, 0, 10)
	val, found := tree.Search("/posts", &params)
	if found || val != nil {
		t.Errorf("expected not found, got found=%v val=%v", found, val)
	}
	if len(params) != 0 {
		t.Errorf("params should be empty after failed search, got %v", params)
	}
}

func TestTree_IntermediateNodeNotEndpoint(t *testing.T) {
	// If only /api/v1/users is registered, searching for /api or /api/v1
	// should return not found — those nodes exist in the tree but have no value.
	tree := New[string]()
	tree.Insert("/api/v1/users", "users")

	params := make(Params, 0, 10)
	_, found := tree.Search("/api", &params)
	if found {
		t.Error("/api should not be found — it's an intermediate node")
	}

	params = params[:0]
	_, found = tree.Search("/api/v1", &params)
	if found {
		t.Error("/api/v1 should not be found — it's an intermediate node")
	}
}

func TestTree_CatchAllStrictMissingSlash(t *testing.T) {
	// A catch-all route /static/*filepath requires the /static/ prefix.
	// Requesting just /static (without trailing slash) should NOT match
	// because the catch-all segment is a separate node that needs at least
	// one more path segment (which can be empty string for /static/).
	tree := New[string]()
	tree.Insert("/static/*filepath", "files")

	params := make(Params, 0, 10)
	_, found := tree.Search("/static", &params)
	if found {
		t.Error("/static (no trailing slash) should not match /static/*filepath")
	}
}

// =============================================================================
// Benchmark — Zero Allocation Verification
// =============================================================================
// The search operation is designed for zero heap allocations when the Params
// slice is pre-allocated with sufficient capacity (via sync.Pool in production).

func BenchmarkTree_Search(b *testing.B) {
	tree := New[string]()
	tree.Insert("/api/v1/users/:id/posts/:postId/comments/*filepath", "handler")

	// Pre-allocate params with capacity 20 (same as production router)
	params := make(Params, 0, 20)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		params = params[:0] // reset length, keep capacity
		tree.Search("/api/v1/users/123/posts/456/comments/docs/readme.md", &params)
	}
}

// =============================================================================
// Helpers
// =============================================================================

func assertSliceEqual(t *testing.T, expect, got []string) {
	t.Helper()
	if len(expect) != len(got) {
		t.Fatalf("length mismatch: expect %v, got %v", expect, got)
	}
	for i := range expect {
		if expect[i] != got[i] {
			t.Errorf("index %d: expect %q, got %q (full: %v vs %v)",
				i, expect[i], got[i], expect, got)
		}
	}
}

// Verify Tree satisfies a basic test for each exported method to catch
// accidental API changes at compile time.
var _ = func() {
	tree := New[string]()
	tree.Insert("/path", "val")
	tree.Upsert("/path", "val", func(e *string, i string) {})
	p := make(Params, 0)
	tree.Search("/path", &p)
	_ = tree.Collect()
	_ = SplitPath("/path")
	_ = fmt.Sprint(p.Get("key"))
}
