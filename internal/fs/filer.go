package fs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxReadSize = 1 << 20 // 1 MB

// ActionRecord logs a single tool execution.
type ActionRecord struct {
	Time   time.Time
	Tool   string
	Args   string
	Result string
}

type Filer struct {
	Base    string
	mu      sync.Mutex
	actions []ActionRecord
}

func (f *Filer) recordAction(tool, args, result string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.actions = append(f.actions, ActionRecord{
		Time:   time.Now(),
		Tool:   tool,
		Args:   args,
		Result: result,
	})
}

// ActionHistory returns a human-readable log of all executed file operations.
func (f *Filer) ActionHistory() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.actions) == 0 {
		return "No file operations have been performed yet."
	}
	var sb strings.Builder
	for i, a := range f.actions {
		fmt.Fprintf(&sb, "%d. [%s] %s(%s) → %s\n",
			i+1,
			a.Time.Format("15:04:05"),
			a.Tool,
			a.Args,
			a.Result,
		)
	}
	return sb.String()
}

func formatSize(size int64) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%d B", size)
	case size < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	case size < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	default:
		return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
	}
}

func dirItemCount(path string) string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "dir"
	}
	return fmt.Sprintf("%d items", len(entries))
}

func New(base string) (*Filer, error) {
	abs, err := filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	return &Filer{Base: abs}, nil
}

func (f *Filer) resolve(rel string) (string, error) {
	if rel == "" || rel == "." || rel == "/" {
		return f.Base, nil
	}
	rel = strings.TrimPrefix(rel, "/")
	target, err := filepath.Abs(filepath.Join(f.Base, rel))
	if err != nil {
		return "", err
	}
	if target != f.Base && !strings.HasPrefix(target, f.Base+string(os.PathSeparator)) {
		return "", errors.New("path escapes Filen mount directory")
	}
	return target, nil
}

func (f *Filer) List(path string) (string, error) {
	target, err := f.resolve(path)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "(empty directory)", nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%-40s  %-10s  %s\n", "name", "size", "modified")
	fmt.Fprintf(&sb, "%s\n", strings.Repeat("─", 65))
	for _, e := range entries {
		name := e.Name()
		var sizeStr string
		if e.IsDir() {
			name += "/"
			sizeStr = dirItemCount(filepath.Join(target, e.Name()))
		} else if info, err := e.Info(); err == nil {
			sizeStr = formatSize(info.Size())
		}
		if info, err := e.Info(); err == nil {
			fmt.Fprintf(&sb, "%-40s  %-10s  %s\n", name, sizeStr, info.ModTime().Format("2006-01-02 15:04"))
		} else {
			fmt.Fprintf(&sb, "%-40s  %-10s\n", name, sizeStr)
		}
	}
	return sb.String(), nil
}

func (f *Filer) ReadFile(path string) (string, error) {
	target, err := f.resolve(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", errors.New("path is a directory; use list_files instead")
	}
	if info.Size() > maxReadSize {
		return "", fmt.Errorf("file too large to read inline (%d bytes); max is %d", info.Size(), maxReadSize)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *Filer) WriteFile(path, content string) (string, error) {
	target, err := f.resolve(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return "", err
	}
	result := fmt.Sprintf("written %d bytes to %s", len(content), path)
	f.recordAction("write_file", path, result)
	return result, nil
}

func (f *Filer) CreateDir(path string) (string, error) {
	target, err := f.resolve(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}
	result := fmt.Sprintf("created directory: %s", path)
	f.recordAction("create_dir", path, result)
	return result, nil
}

func (f *Filer) Delete(path string) (string, error) {
	target, err := f.resolve(path)
	if err != nil {
		return "", err
	}
	if target == f.Base {
		return "", errors.New("cannot delete the Filen mount root")
	}
	if err := os.RemoveAll(target); err != nil {
		return "", err
	}
	result := fmt.Sprintf("deleted: %s", path)
	f.recordAction("delete", path, result)
	return result, nil
}

func (f *Filer) Move(src, dst string) (string, error) {
	srcAbs, err := f.resolve(src)
	if err != nil {
		return "", err
	}
	dstAbs, err := f.resolve(dst)
	if err != nil {
		return "", err
	}
	// Ensure destination stays within the base directory.
	if !strings.HasPrefix(dstAbs, f.Base+string(os.PathSeparator)) && dstAbs != f.Base {
		return "", errors.New("destination path escapes Filen mount directory")
	}
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(srcAbs, dstAbs); err != nil {
		return "", err
	}
	result := fmt.Sprintf("moved %s → %s", src, dst)
	f.recordAction("move", fmt.Sprintf("%s → %s", src, dst), result)
	return result, nil
}

func (f *Filer) Copy(src, dst string) (string, error) {
	srcAbs, err := f.resolve(src)
	if err != nil {
		return "", err
	}
	dstAbs, err := f.resolve(dst)
	if err != nil {
		return "", err
	}
	// Ensure destination stays within the base directory.
	if !strings.HasPrefix(dstAbs, f.Base+string(os.PathSeparator)) && dstAbs != f.Base {
		return "", errors.New("destination path escapes Filen mount directory")
	}
	srcFile, err := os.Open(srcAbs)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", errors.New("cannot copy a directory")
	}
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return "", err
	}
	dstFile, err := os.OpenFile(dstAbs, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		return "", err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return "", err
	}
	result := fmt.Sprintf("copied %s → %s", src, dst)
	f.recordAction("copy", fmt.Sprintf("%s → %s", src, dst), result)
	return result, nil
}

// Tree returns a visual tree of the Filen mount up to maxDepth levels.
func (f *Filer) Tree(maxDepth int) string {
	var sb strings.Builder
	walkTree(&sb, f.Base, "", 0, maxDepth)
	result := sb.String()
	if result == "" {
		return "(empty)"
	}
	return result
}

func walkTree(sb *strings.Builder, path, indent string, depth, maxDepth int) {
	if depth > maxDepth {
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for i, e := range entries {
		isLast := i == len(entries)-1
		prefix := "├── "
		childIndent := indent + "│   "
		if isLast {
			prefix = "└── "
			childIndent = indent + "    "
		}
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		fmt.Fprintf(sb, "%s%s%s\n", indent, prefix, name)
		if e.IsDir() && depth < maxDepth {
			walkTree(sb, filepath.Join(path, e.Name()), childIndent, depth+1, maxDepth)
		}
	}
}
