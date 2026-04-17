package filer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helpers

func newTestLocal(t *testing.T) (*LocalFiler, string) {
	t.Helper()
	dir := t.TempDir()
	f, err := NewLocal(dir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	return f, dir
}

// ── Sandbox / path-traversal ──────────────────────────────────────────────────

func TestResolve_BlocksTraversal(t *testing.T) {
	f, _ := newTestLocal(t)

	cases := []struct {
		name string
		rel  string
	}{
		{"dotdot prefix", "../secret"},
		{"dotdot deep", "a/../../secret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.resolve(tc.rel)
			if err == nil {
				t.Errorf("resolve(%q) should have returned an error", tc.rel)
			}
		})
	}
}

func TestResolve_AllowsValidPaths(t *testing.T) {
	f, _ := newTestLocal(t)

	cases := []string{".", "", "/", "a/b/c", "file.txt"}
	for _, rel := range cases {
		t.Run(rel, func(t *testing.T) {
			got, err := f.resolve(rel)
			if err != nil {
				t.Errorf("resolve(%q) unexpected error: %v", rel, err)
			}
			if !strings.HasPrefix(got, f.Base) {
				t.Errorf("resolve(%q) = %q, not under base %q", rel, got, f.Base)
			}
		})
	}
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestList_EmptyDir(t *testing.T) {
	f, _ := newTestLocal(t)
	out, err := f.List(".")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if out != "(empty directory)" {
		t.Errorf("expected empty message, got %q", out)
	}
}

func TestList_WithFiles(t *testing.T) {
	f, dir := newTestLocal(t)
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0o644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)

	out, err := f.List(".")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !strings.Contains(out, "hello.txt") {
		t.Errorf("listing missing hello.txt:\n%s", out)
	}
	if !strings.Contains(out, "subdir/") {
		t.Errorf("listing missing subdir/:\n%s", out)
	}
}

// ── ReadFile ──────────────────────────────────────────────────────────────────

func TestReadFile(t *testing.T) {
	f, dir := newTestLocal(t)
	want := "hello world"
	os.WriteFile(filepath.Join(dir, "note.txt"), []byte(want), 0o644)

	got, err := f.ReadFile("note.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got != want {
		t.Errorf("ReadFile = %q, want %q", got, want)
	}
}

func TestReadFile_Directory(t *testing.T) {
	f, dir := newTestLocal(t)
	os.Mkdir(filepath.Join(dir, "mydir"), 0o755)

	_, err := f.ReadFile("mydir")
	if err == nil {
		t.Error("expected error reading a directory")
	}
}

// ── WriteFile ─────────────────────────────────────────────────────────────────

func TestWriteFile(t *testing.T) {
	f, dir := newTestLocal(t)
	content := "test content"
	_, err := f.WriteFile("out.txt", content)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "out.txt"))
	if string(data) != content {
		t.Errorf("file content = %q, want %q", data, content)
	}
}

func TestWriteFile_CreatesParents(t *testing.T) {
	f, dir := newTestLocal(t)
	_, err := f.WriteFile("a/b/c.txt", "nested")
	if err != nil {
		t.Fatalf("WriteFile nested: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a/b/c.txt")); err != nil {
		t.Errorf("nested file not created: %v", err)
	}
}

// ── CreateDir ─────────────────────────────────────────────────────────────────

func TestCreateDir(t *testing.T) {
	f, dir := newTestLocal(t)
	_, err := f.CreateDir("newdir/sub")
	if err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "newdir/sub")); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestDelete(t *testing.T) {
	f, dir := newTestLocal(t)
	path := filepath.Join(dir, "todelete.txt")
	os.WriteFile(path, []byte("bye"), 0o644)

	_, err := f.Delete("todelete.txt")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestDelete_Root(t *testing.T) {
	f, _ := newTestLocal(t)
	_, err := f.Delete(".")
	if err == nil {
		t.Error("expected error deleting the Filen mount root")
	}
}

// ── Move ──────────────────────────────────────────────────────────────────────

func TestMove(t *testing.T) {
	f, dir := newTestLocal(t)
	os.WriteFile(filepath.Join(dir, "src.txt"), []byte("data"), 0o644)

	_, err := f.Move("src.txt", "dst.txt")
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src.txt")); !os.IsNotExist(err) {
		t.Error("source should not exist after move")
	}
	if _, err := os.Stat(filepath.Join(dir, "dst.txt")); err != nil {
		t.Errorf("destination should exist: %v", err)
	}
}

// ── Copy ──────────────────────────────────────────────────────────────────────

func TestCopy(t *testing.T) {
	f, dir := newTestLocal(t)
	os.WriteFile(filepath.Join(dir, "orig.txt"), []byte("copy me"), 0o644)

	_, err := f.Copy("orig.txt", "copy.txt")
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "copy.txt"))
	if string(data) != "copy me" {
		t.Errorf("copy content = %q", data)
	}
	// Original should still exist.
	if _, err := os.Stat(filepath.Join(dir, "orig.txt")); err != nil {
		t.Error("original should still exist after copy")
	}
}

// ── ActionHistory ─────────────────────────────────────────────────────────────

func TestActionHistory_Empty(t *testing.T) {
	f, _ := newTestLocal(t)
	h := f.ActionHistory()
	if h != "No file operations have been performed yet." {
		t.Errorf("unexpected empty history: %q", h)
	}
}

func TestActionHistory_Records(t *testing.T) {
	f, _ := newTestLocal(t)
	f.WriteFile("x.txt", "hello") // write_file records an action

	h := f.ActionHistory()
	if !strings.Contains(h, "write_file") {
		t.Errorf("history should contain write_file:\n%s", h)
	}
}

// ── Tree ──────────────────────────────────────────────────────────────────────

func TestTree_Empty(t *testing.T) {
	f, _ := newTestLocal(t)
	if got := f.Tree(3); got != "(empty)" {
		t.Errorf("Tree on empty dir = %q", got)
	}
}

func TestTree_WithFiles(t *testing.T) {
	f, dir := newTestLocal(t)
	os.Mkdir(filepath.Join(dir, "a"), 0o755)
	os.WriteFile(filepath.Join(dir, "a", "file.txt"), []byte("hi"), 0o644)

	out := f.Tree(3)
	if !strings.Contains(out, "a/") {
		t.Errorf("tree missing dir a/:\n%s", out)
	}
	if !strings.Contains(out, "file.txt") {
		t.Errorf("tree missing file.txt:\n%s", out)
	}
}
