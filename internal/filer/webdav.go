package filer

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/studio-b12/gowebdav"
)

// maxReadSize is the maximum file size (1 MB) the Filer will read inline.
const maxReadSize = 1 << 20

// ActionRecord logs a single tool execution.
type ActionRecord struct {
	Time   time.Time
	Tool   string
	Args   string
	Result string
}

// WebDAVFiler implements Filer backed by a Filen WebDAV server.
type WebDAVFiler struct {
	client   *gowebdav.Client
	user     string
	pass     string
	rootPath string
	baseURL  string
	mu       sync.Mutex
	actions  []ActionRecord
}

// NewWebDAV creates a new WebDAV-backed Filer that connects to the given
// webdavURL with the provided credentials.
func NewWebDAV(webdavURL, user, pass string) (*WebDAVFiler, error) {
	parsedURL, err := url.Parse(webdavURL)
	if err != nil {
		return nil, fmt.Errorf("invalid WebDAV URL: %w", err)
	}
	if parsedURL.Path == "" || parsedURL.Path == "/" {
		parsedURL.Path = "/"
	}

	c := gowebdav.NewClient(webdavURL, user, pass)

	return &WebDAVFiler{
		client:   c,
		user:     user,
		pass:     pass,
		rootPath: parsedURL.Path,
		baseURL:  webdavURL,
	}, nil
}

func (f *WebDAVFiler) recordAction(tool, args, result string) {
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
func (f *WebDAVFiler) ActionHistory() string {
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

func formatWebDAVSize(size int64) string {
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

// resolvePath converts a relative path to an absolute WebDAV path.
func (f *WebDAVFiler) resolvePath(rel string) string {
	if rel == "" || rel == "." || rel == "/" {
		return f.rootPath
	}
	rel = strings.TrimPrefix(rel, "/")
	return filepath.Join(f.rootPath, rel)
}

// containsPath reports whether target is f.rootPath or a descendant of it.
// Using a prefix check without a trailing slash separator allows sibling paths
// (e.g. rootPath="/foo" would incorrectly accept "/foobar"), so we require the
// separator between rootPath and the rest of the resolved path.
func (f *WebDAVFiler) containsPath(resolved string) bool {
	return resolved == f.rootPath || strings.HasPrefix(resolved, f.rootPath+"/")
}

// validatePath ensures the path stays within the root.
func (f *WebDAVFiler) validatePath(path string) error {
	resolved := f.resolvePath(path)
	if !f.containsPath(resolved) {
		return fmt.Errorf("path escapes WebDAV root")
	}
	return nil
}

// absURL returns the absolute URL for a given path.
func (f *WebDAVFiler) absURL(path string) string {
	return f.baseURL + path
}

// Ping implements Filer.
func (f *WebDAVFiler) Ping() error {
	_, err := f.client.ReadDir("/")
	if err != nil {
		return fmt.Errorf("WebDAV server not reachable: %w", err)
	}
	return nil
}

// List implements Filer.
func (f *WebDAVFiler) List(path string) (string, error) {
	if err := f.validatePath(path); err != nil {
		return "", err
	}
	absPath := f.resolvePath(path)
	entries, err := f.client.ReadDir(absPath)
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
		if e.IsDir() {
			name += "/"
			subEntries, subErr := f.client.ReadDir(filepath.Join(absPath, e.Name()))
			if subErr == nil {
				sizeStr = fmt.Sprintf("%d items", len(subEntries))
			} else {
				sizeStr = "dir"
			}
		} else {
			sizeStr = formatWebDAVSize(e.Size())
		}
		if !e.ModTime().IsZero() {
			modTime = e.ModTime().Format("2006-01-02 15:04")
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
func (f *WebDAVFiler) ReadFile(path string) (string, error) {
	if err := f.validatePath(path); err != nil {
		return "", err
	}
	absPath := f.resolvePath(path)

	info, err := f.client.Stat(absPath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory; use list_files instead")
	}

	if info.Size() > maxReadSize {
		return "", fmt.Errorf("file too large to read inline (%d bytes); max is %d", info.Size(), maxReadSize)
	}

	data, err := f.client.Read(absPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile implements Filer.
func (f *WebDAVFiler) WriteFile(path, content string) (string, error) {
	if err := f.validatePath(path); err != nil {
		return "", err
	}
	absPath := f.resolvePath(path)
	reader := bytes.NewReader([]byte(content))
	err := f.client.WriteStream(absPath, reader, 0o644)
	if err != nil {
		return "", err
	}
	result := fmt.Sprintf("written %d bytes to %s", len(content), path)
	f.recordAction("write_file", path, result)
	return result, nil
}

// CreateDir implements Filer.
func (f *WebDAVFiler) CreateDir(path string) (string, error) {
	if err := f.validatePath(path); err != nil {
		return "", err
	}
	absPath := f.resolvePath(path)
	err := f.client.MkdirAll(absPath, 0o755)
	if err != nil {
		return "", err
	}
	result := fmt.Sprintf("created directory: %s", path)
	f.recordAction("create_dir", path, result)
	return result, nil
}

// Delete implements Filer.
func (f *WebDAVFiler) Delete(path string) (string, error) {
	if err := f.validatePath(path); err != nil {
		return "", err
	}
	absPath := f.resolvePath(path)
	err := f.client.RemoveAll(absPath)
	if err != nil {
		return "", err
	}
	result := fmt.Sprintf("deleted: %s", path)
	f.recordAction("delete", path, result)
	return result, nil
}

// Move implements Filer.
func (f *WebDAVFiler) Move(src, dst string) (string, error) {
	srcAbs := f.resolvePath(src)
	dstAbs := f.resolvePath(dst)
	// Validate resolved paths to catch absolute-path escapes.
	// A bare HasPrefix("/foobar", "/foo") would be a false positive, so we
	// require an exact match or a path separator after the root.
	if !f.containsPath(srcAbs) {
		return "", fmt.Errorf("path escapes WebDAV root: %s", src)
	}
	if !f.containsPath(dstAbs) {
		return "", fmt.Errorf("path escapes WebDAV root: %s", dst)
	}

	parent := filepath.Dir(dstAbs)
	if parent != f.rootPath {
		if err := f.client.MkdirAll(parent, 0o755); err != nil {
			return "", fmt.Errorf("cannot create destination parent directory: %w", err)
		}
	}

	// Use raw HTTP MOVE with credentials forwarded from the configured client.
	req, err := http.NewRequest("MOVE", f.absURL(srcAbs), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Destination", f.absURL(dstAbs))
	req.Header.Set("Overwrite", "T")
	if f.user != "" {
		req.SetBasicAuth(f.user, f.pass)
	}

	client := &http.Client{}
	client.Transport = &http.Transport{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("webdav request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 && resp.StatusCode != 201 && resp.StatusCode != 204 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("webdav MOVE failed with HTTP %d: %s", resp.StatusCode, string(body))
	}
	result := fmt.Sprintf("moved %s → %s", src, dst)
	f.recordAction("move", fmt.Sprintf("%s → %s", src, dst), result)
	return result, nil
}

// Copy implements Filer.
func (f *WebDAVFiler) Copy(src, dst string) (string, error) {
	srcAbs := f.resolvePath(src)
	dstAbs := f.resolvePath(dst)
	// Validate resolved paths to catch absolute-path escapes.
	if !f.containsPath(srcAbs) {
		return "", fmt.Errorf("path escapes WebDAV root: %s", src)
	}
	if !f.containsPath(dstAbs) {
		return "", fmt.Errorf("path escapes WebDAV root: %s", dst)
	}

	info, err := f.client.Stat(srcAbs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("cannot copy a directory")
	}

	parent := filepath.Dir(dstAbs)
	if parent != f.rootPath {
		if err := f.client.MkdirAll(parent, 0o755); err != nil {
			return "", fmt.Errorf("cannot create destination parent directory: %w", err)
		}
	}

	data, err := f.client.Read(srcAbs)
	if err != nil {
		return "", err
	}

	reader := bytes.NewReader(data)
	err = f.client.WriteStream(dstAbs, reader, 0o644)
	if err != nil {
		return "", err
	}
	result := fmt.Sprintf("copied %s → %s", src, dst)
	f.recordAction("copy", fmt.Sprintf("%s → %s", src, dst), result)
	return result, nil
}

// Tree implements Filer.
func (f *WebDAVFiler) Tree(maxDepth int) string {
	var sb strings.Builder
	f.walkTree(&sb, f.rootPath, "", 0, maxDepth)
	result := sb.String()
	if result == "" {
		return "(empty)"
	}
	return result
}

// TreeWithPath implements Filer.
func (f *WebDAVFiler) TreeWithPath(path string, maxDepth int) string {
	if err := f.validatePath(path); err != nil {
		return "Error: " + err.Error()
	}
	absPath := f.resolvePath(path)
	var sb strings.Builder
	f.walkTree(&sb, absPath, "", 0, maxDepth)
	result := sb.String()
	if result == "" {
		return "(empty)"
	}
	return result
}

// ListFiles implements Filer.
func (f *WebDAVFiler) ListFiles(path string) ([]File, error) {
	if err := f.validatePath(path); err != nil {
		return nil, err
	}
	absPath := f.resolvePath(path)
	entries, err := f.client.ReadDir(absPath)
	if err != nil {
		return nil, err
	}
	var files []File
	for _, e := range entries {
		files = append(files, File{
			Name:     e.Name(),
			Size:     e.Size(),
			IsDir:    e.IsDir(),
			Modified: e.ModTime(),
		})
	}
	return files, nil
}

func (f *WebDAVFiler) walkTree(sb *strings.Builder, path, indent string, depth, maxDepth int) {
	if depth > maxDepth {
		return
	}
	entries, err := f.client.ReadDir(path)
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
			f.walkTree(sb, filepath.Join(path, e.Name()), childIndent, depth+1, maxDepth)
		}
	}
}
