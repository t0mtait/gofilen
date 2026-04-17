package filer

import "testing"

// TestContainsPath verifies the path-containment helper used by validatePath,
// Move, and Copy to prevent path-traversal escapes via sibling paths.
//
// The classic bug: strings.HasPrefix("/foobar", "/foo") == true, so "/foobar"
// would pass a naive prefix check when rootPath is "/foo".  Our containsPath
// helper requires a separator after the root prefix.
func TestContainsPath(t *testing.T) {
	f := &WebDAVFiler{rootPath: "/foo"}

	cases := []struct {
		resolved string
		want     bool
	}{
		// Exact match — the root itself.
		{"/foo", true},
		// Direct child.
		{"/foo/bar", true},
		// Nested child.
		{"/foo/bar/baz.txt", true},
		// Sibling that starts with root prefix — must be rejected.
		{"/foobar", false},
		{"/foobar/secret", false},
		// Completely outside.
		{"/etc/passwd", false},
		{"/", false},
	}

	for _, tc := range cases {
		got := f.containsPath(tc.resolved)
		if got != tc.want {
			t.Errorf("containsPath(%q) = %v, want %v", tc.resolved, got, tc.want)
		}
	}
}

// TestValidatePath exercises validatePath end-to-end with a concrete root.
func TestValidatePath(t *testing.T) {
	f := &WebDAVFiler{rootPath: "/remote"}

	// Paths that should be accepted.
	for _, p := range []string{".", "", "/", "docs/report.pdf", "a/b/c"} {
		if err := f.validatePath(p); err != nil {
			t.Errorf("validatePath(%q) unexpected error: %v", p, err)
		}
	}

	// Paths that traverse outside the root.
	for _, p := range []string{"../escape", "a/../../escape"} {
		if err := f.validatePath(p); err == nil {
			t.Errorf("validatePath(%q) should have returned an error", p)
		}
	}
}
