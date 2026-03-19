package refl

import (
	"reflect"
	"testing"
)

type TestStruct struct {
	Name       string `json:"name"`
	Age        int    `json:"age,omitempty"`
	Ignored    string `json:"-"`
	unexported string `json:"unexported"`
	NoTag      string
	Email      string `json:"email"`
}

func TestExtractFields(t *testing.T) {
	tests := []struct {
		name       string
		input      any
		targetTags []string
		expected   map[string]any
	}{
		{
			name: "extract specific fields",
			input: TestStruct{
				Name:       "John Doe",
				Age:        30,
				Ignored:    "ignore me",
				unexported: "hidden",
				NoTag:      "no tag",
				Email:      "john@example.com",
			},
			targetTags: []string{"name", "age"},
			expected: map[string]any{
				"name": "John Doe",
				"age":  30,
			},
		},
		{
			name: "with pointer to struct",
			input: &TestStruct{
				Name: "Jane Doe",
				Age:  25,
			},
			targetTags: []string{"name", "age", "email"},
			expected: map[string]any{
				"name":  "Jane Doe",
				"age":   25,
				"email": "",
			},
		},
		{
			name: "ignored and unexported fields",
			input: TestStruct{
				Name:       "Test",
				Ignored:    "ignored",
				unexported: "unexported",
				NoTag:      "no tag",
			},
			targetTags: []string{"-", "unexported", "NoTag", "name"},
			expected: map[string]any{
				"name": "Test",
			},
		},
		{
			name:       "not a struct",
			input:      "just a string",
			targetTags: []string{"name"},
			expected:   map[string]any{},
		},
		{
			name:       "nil input",
			input:      nil,
			targetTags: []string{"name"},
			expected:   map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractFields(tt.input, tt.targetTags)
			if len(result) == 0 && len(tt.expected) == 0 {
				return // Both empty maps are equivalent
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ExtractFields() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
