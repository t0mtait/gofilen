package llm

import (
	"encoding/json"
	"fmt"

	"github.com/t0mtait/gofilen/internal/fs"
)

// FileTools returns the tool definitions the LLM can use to manage the Filen drive.
func FileTools() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "list_files",
				Description: "List files and directories at the given path in the Filen cloud drive.",
				Parameters: ToolParameters{
					Type:     "object",
					Required: []string{"path"},
					Properties: map[string]PropertySchema{
						"path": {Type: "string", Description: "Directory path relative to the Filen drive root. Use '.' for the root."},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "read_file",
				Description: "Read and return the text contents of a file from the Filen cloud drive.",
				Parameters: ToolParameters{
					Type:     "object",
					Required: []string{"path"},
					Properties: map[string]PropertySchema{
						"path": {Type: "string", Description: "File path relative to the Filen drive root."},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "write_file",
				Description: "Create or overwrite a file in the Filen cloud drive with the given text content.",
				Parameters: ToolParameters{
					Type:     "object",
					Required: []string{"path", "content"},
					Properties: map[string]PropertySchema{
						"path":    {Type: "string", Description: "File path to create or overwrite."},
						"content": {Type: "string", Description: "Text content to write."},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "create_dir",
				Description: "Create a directory (and any missing parents) in the Filen cloud drive.",
				Parameters: ToolParameters{
					Type:     "object",
					Required: []string{"path"},
					Properties: map[string]PropertySchema{
						"path": {Type: "string", Description: "Directory path to create."},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "delete",
				Description: "Delete a file or directory (and all its contents) from the Filen cloud drive.",
				Parameters: ToolParameters{
					Type:     "object",
					Required: []string{"path"},
					Properties: map[string]PropertySchema{
						"path": {Type: "string", Description: "Path of the file or directory to delete."},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "move",
				Description: "Move or rename a file or directory within the Filen cloud drive.",
				Parameters: ToolParameters{
					Type:     "object",
					Required: []string{"src", "dst"},
					Properties: map[string]PropertySchema{
						"src": {Type: "string", Description: "Source path."},
						"dst": {Type: "string", Description: "Destination path."},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "copy",
				Description: "Copy a file within the Filen cloud drive.",
				Parameters: ToolParameters{
					Type:     "object",
					Required: []string{"src", "dst"},
					Properties: map[string]PropertySchema{
						"src": {Type: "string", Description: "Source file path."},
						"dst": {Type: "string", Description: "Destination file path."},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_action_history",
				Description: "Return a log of all file operations performed this session. Call this when the user asks what you have done, your history, or what changed.",
				Parameters: ToolParameters{
					Type:       "object",
					Required:   []string{},
					Properties: map[string]PropertySchema{},
				},
			},
		},
	}
}

// IsDestructive reports whether a tool modifies, creates, or deletes files.
func IsDestructive(name string) bool {
	switch name {
	case "write_file", "create_dir", "delete", "move", "copy":
		return true
	}
	return false
}

// ExecuteTool dispatches a tool call to the underlying Filer.
func ExecuteTool(name string, argsRaw json.RawMessage, filer *fs.Filer) (string, error) {
	var args map[string]string
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return "", fmt.Errorf("invalid tool arguments: %w", err)
	}

	switch name {
	case "list_files":
		if args["path"] == "" {
			return "", fmt.Errorf("list_files requires 'path' argument")
		}
		return filer.List(args["path"])
	case "read_file":
		if args["path"] == "" {
			return "", fmt.Errorf("read_file requires 'path' argument")
		}
		return filer.ReadFile(args["path"])
	case "write_file":
		if args["path"] == "" {
			return "", fmt.Errorf("write_file requires 'path' argument")
		}
		if args["content"] == "" {
			return "", fmt.Errorf("write_file requires 'content' argument")
		}
		return filer.WriteFile(args["path"], args["content"])
	case "create_dir":
		if args["path"] == "" {
			return "", fmt.Errorf("create_dir requires 'path' argument")
		}
		return filer.CreateDir(args["path"])
	case "delete":
		if args["path"] == "" {
			return "", fmt.Errorf("delete requires 'path' argument")
		}
		return filer.Delete(args["path"])
	case "move":
		if args["src"] == "" || args["dst"] == "" {
			return "", fmt.Errorf("move requires 'src' and 'dst' arguments")
		}
		return filer.Move(args["src"], args["dst"])
	case "copy":
		if args["src"] == "" || args["dst"] == "" {
			return "", fmt.Errorf("copy requires 'src' and 'dst' arguments")
		}
		return filer.Copy(args["src"], args["dst"])
	case "get_action_history":
		return filer.ActionHistory(), nil
	default:
		return "", fmt.Errorf("unknown tool: %q", name)
	}
}
