package router

import (
	"reflect"
	"testing"
)

func TestParsePath(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		expected    []pathPart
		expectError bool
	}{
		{
			name:  "static only",
			input: "/users/profile",
			expected: []pathPart{
				{path: "/users/profile", kind: nodeStatic},
			},
		},
		{
			name:  "single param",
			input: "/users/:id",
			expected: []pathPart{
				{path: "/users/", kind: nodeStatic},
				{path: ":id", kind: nodeParam},
			},
		},
		{
			name:  "multiple params",
			input: "/users/:id/posts/:postId",
			expected: []pathPart{
				{path: "/users/", kind: nodeStatic},
				{path: ":id", kind: nodeParam},
				{path: "/posts/", kind: nodeStatic},
				{path: ":postId", kind: nodeParam},
			},
		},
		{
			name:  "wildcard at end",
			input: "/files/*path",
			expected: []pathPart{
				{path: "/files/", kind: nodeStatic},
				{path: "*path", kind: nodeWildcard},
			},
		},
		{
			name:        "wildcard not at end",
			input:       "/files/*path/extra",
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePath(tc.input)

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected an error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("got %+v, want %+v", got, tc.expected)
			}
		})
	}
}

func TestValidateUniquePath(t *testing.T) {
	var parts []pathPart
	parts = append(parts, pathPart{path: "users/", kind: nodeStatic})
	parts = append(parts, pathPart{path: ":id", kind: nodeParam})
	parts = append(parts, pathPart{path: "/orders/", kind: nodeStatic})
	parts = append(parts, pathPart{path: ":orderId", kind: nodeParam})
	parts = append(parts, pathPart{path: "/item/", kind: nodeStatic})
	parts = append(parts, pathPart{path: ":id", kind: nodeParam})

	err := validateUniqueNames(parts)

	if err == nil {
		t.Fatal("expected error")
	}
}
