package pathnorm

import (
	"testing"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern string
		file    string
		want    bool
	}{
		// Exact match.
		{"src/auth/login.go", "src/auth/login.go", true},
		{"src/auth/login.go", "src/auth/signup.go", false},

		// Directory prefix.
		{"src/auth", "src/auth/login.go", true},
		{"src/auth", "src/auth/handlers/oauth.go", true},
		{"src/auth", "src/api/login.go", false},

		// Glob patterns.
		{"src/auth/*.go", "src/auth/login.go", true},
		{"src/auth/*.go", "src/auth/login.ts", false},
		{"src/*.go", "src/main.go", true},

		// No match.
		{"src/api", "src/auth/login.go", false},
	}

	for _, tt := range tests {
		got := Match(tt.pattern, tt.file)
		if got != tt.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.file, got, tt.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	// Use current directory as a stand-in for repo root.
	// Normalize should return a clean relative path.
	norm, err := Normalize("./foo/../bar/baz.go", ".")
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if norm != "bar/baz.go" {
		t.Errorf("Normalize(./foo/../bar/baz.go) = %q, want %q", norm, "bar/baz.go")
	}
}
