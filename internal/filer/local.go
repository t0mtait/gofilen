package filer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LocalFiler is the original filesystem-backed Filer.
type LocalFiler struct {
	Base    string
	mu      sync.Mutex
	actions []ActionRecord
}

func NewLocal(base string) (*LocalFiler, error) {
	abs, err := filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	return &LocalFiler{Base: abs}, nil
}

func (f *LocalFiler) recordAction(tool, args, result string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.actions = append(f.actions, ActionRecord{
		Time:   time.Now(),
		Tool:   tool,
		Args:   args,
		Result: result,
	})
}

// ActionHistory implements Filer.
func (f *LocalFiler) ActionHistory() string {
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

func formatLocalSize(size int64) string {
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

func localDirItemCount(path string) string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "dir"
	}
	return fmt.Sprintf("%d items", len(entries))
}

func (f *LocalFiler) resolve(rel string) (string, error) {
	if rel == "" || rel == "." || rel == "/" {
		return f.Base, nil
	}
	rel = strings.TrimPrefix(rel, "/")
	abs := filepath.Join(f.Base, rel)

	realAbs, err := filepath.EvalSymlinks(abs)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err != nil {
		realAbs = abs
	}

	if realAbs != f.Base && !strings.HasPrefix(realAbs, f.Base+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes Filen mount directory")
	}
	return realAbs, nil
}

// Ping implements Filer.
func (f *LocalFiler) Ping() error {
	_, err := os.Stat(f.Base)
	return err
}

// List implements Filer.
func (f *LocalFiler) List(path string) (string, error) {
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
		var modTime string
		if info, err := e.Info(); err == nil {
			if e.IsDir() {
				name += "/"
				sizeStr = localDirItemCount(filepath.Join(target, e.Name()))
			} else {
				sizeStr = formatLocalSize(info.Size())
			}
			modTime = info.ModTime().Format("2006-01-02 15:04")
		} else if e.IsDir() {
			name += "/"
			sizeStr = localDirItemCount(filepath.Join(target, e.Name()))
		}
		if modTime != "" {
			fmt.Fprintf(&sb, "%-40s  %-10s  %s\n", name, sizeStr, modTime)
		} else {
			fmt.Fprintf(&sb, "%-40s  %-10s\n", name, sizeStr)
		}
	}
	return sb.String(), nil
}

// ReadFile implements Filer.
func (f *LocalFiler) ReadFile(path string) (string, error) {
	target, err := f.resolve(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory; use list_files instead")
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

// WriteFile implements Filer.
func (f *LocalFiler) WriteFile(path, content string) (string, error) {
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

// CreateDir implements Filer.
func (f *LocalFiler) CreateDir(path string) (string, error) {
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

// Delete implements Filer.
func (f *LocalFiler) Delete(path string) (string, error) {
	target, err := f.resolve(path)
	if err != nil {
		return "", err
	}
	if target == f.Base {
		return "", fmt.Errorf("cannot delete the Filen mount root")
	}
	if err := os.RemoveAll(target); err != nil {
		return "", err
	}
	result := fmt.Sprintf("deleted: %s", path)
	f.recordAction("delete", path, result)
	return result, nil
}

// Move implements Filer.
func (f *LocalFiler) Move(src, dst string) (string, error) {
	srcAbs, err := f.resolve(src)
	if err != nil {
		return "", err
	}
	dstAbs, err := f.resolve(dst)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(dstAbs, f.Base+string(os.PathSeparator)) && dstAbs != f.Base {
		return "", fmt.Errorf("path escapes Filen mount directory")
	}
	if filepath.Dir(dstAbs) != f.Base {
		if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
			return "", err
		}
	}
	if err := os.Rename(srcAbs, dstAbs); err != nil {
		return "", err
	}
	result := fmt.Sprintf("moved %s → %s", src, dst)
	f.recordAction("move", fmt.Sprintf("%s → %s", src, dst), result)
	return result, nil
}

// Copy implements Filer.
func (f *LocalFiler) Copy(src, dst string) (string, error) {
	srcAbs, err := f.resolve(src)
	if err != nil {
		return "", err
	}
	dstAbs, err := f.resolve(dst)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(dstAbs, f.Base+string(os.PathSeparator)) && dstAbs != f.Base {
		return "", fmt.Errorf("path escapes Filen mount directory")
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
		return "", fmt.Errorf("cannot copy a directory")
	}
	if filepath.Dir(dstAbs) != f.Base {
		if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
			return "", err
		}
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

// ListFiles implements Filer.
func (f *LocalFiler) ListFiles(path string) ([]File, error) {
	target, err := f.resolve(path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, File{
			Name:     e.Name(),
			Size:     info.Size(),
			IsDir:    e.IsDir(),
			Modified: info.ModTime(),
		})
	}
	return files, nil
}

// Tree implements Filer.
func (f *LocalFiler) Tree(maxDepth int) string {
	var sb strings.Builder
	walkTreeLocal(&sb, f.Base, "", 0, maxDepth)
	result := sb.String()
	if result == "" {
		return "(empty)"
	}
	return result
}

func walkTreeLocal(sb *strings.Builder, path, indent string, depth, maxDepth int) {
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
			walkTreeLocal(sb, filepath.Join(path, e.Name()), childIndent, depth+1, maxDepth)
		}
	}
}
