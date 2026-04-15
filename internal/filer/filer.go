package filer

// Filer is the interface for accessing a Filen cloud drive.
// Both the local filesystem-backed implementation and the WebDAV-backed
// implementation satisfy this interface.
type Filer interface {
	// List returns a formatted directory listing for the given path.
	List(path string) (string, error)
	// ReadFile returns the text content of a file at the given path.
	ReadFile(path string) (string, error)
	// WriteFile writes content to a file at the given path.
	WriteFile(path, content string) (string, error)
	// CreateDir creates a directory (and any missing parents) at the given path.
	CreateDir(path string) (string, error)
	// Delete removes a file or directory at the given path.
	Delete(path string) (string, error)
	// Move renames or moves a file or directory from src to dst.
	Move(src, dst string) (string, error)
	// Copy copies a file from src to dst.
	Copy(src, dst string) (string, error)
	// Tree returns a visual tree of the directory up to maxDepth levels.
	Tree(maxDepth int) string
	// ActionHistory returns a human-readable log of all executed file operations.
	ActionHistory() string
	// Ping checks connectivity to the underlying storage.
	Ping() error
}