package llm

import (
	"encoding/json"
	"fmt"

	"github.com/t0mtait/gofilen/internal/filer"
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
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "tree",
				Description: "Show a tree view of a Filen directory up to a given depth (default 3, max 10). Use this to explore the directory structure when the user asks to see the folder hierarchy.",
				Parameters: ToolParameters{
					Type:     "object",
					Required: []string{},
					Properties: map[string]PropertySchema{
						"path":  {Type: "string", Description: "Directory path relative to the Filen drive root. Use '.' for the root. Defaults to root if not specified."},
						"depth": {Type: "integer", Description: "Tree depth to display (1-10, default 3). Higher values show more nested directories."},
					},
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
func ExecuteTool(name string, argsRaw json.RawMessage, f filer.Filer) (string, error) {
	switch name {
	case "list_files", "read_file", "write_file", "create_dir", "delete", "move", "copy":
		var args map[string]string
		if err := json.Unmarshal(argsRaw, &args); err != nil {
			return "", fmt.Errorf("invalid tool arguments: %w", err)
		}
		return executeFileOp(name, args, f)
	case "get_action_history":
		return f.ActionHistory(), nil
	case "tree":
		var treeArgs struct {
			Path  string `json:"path"`
			Depth *int   `json:"depth"`
		}
		if err := json.Unmarshal(argsRaw, &treeArgs); err != nil {
			return "", fmt.Errorf("invalid tree arguments: %w", err)
		}
		depth := 3
		if treeArgs.Depth != nil && *treeArgs.Depth >= 1 && *treeArgs.Depth <= 10 {
			depth = *treeArgs.Depth
		}
		targetPath := treeArgs.Path
		if targetPath == "" {
			targetPath = "."
		}
		return f.TreeWithPath(targetPath, depth), nil
	default:
		return "", fmt.Errorf("unknown tool: %q", name)
	}
}

// executeFileOp handles file operation tools that take string arguments.
func executeFileOp(name string, args map[string]string, f filer.Filer) (string, error) {
	switch name {
	case "list_files":
		if args["path"] == "" {
			return "", fmt.Errorf("list_files requires 'path' argument")
		}
		return f.List(args["path"])
	case "read_file":
		if args["path"] == "" {
			return "", fmt.Errorf("read_file requires 'path' argument")
		}
		return f.ReadFile(args["path"])
	case "write_file":
		if args["path"] == "" {
			return "", fmt.Errorf("write_file requires 'path' argument")
		}
		if args["content"] == "" {
			return "", fmt.Errorf("write_file requires 'content' argument")
		}
		return f.WriteFile(args["path"], args["content"])
	case "create_dir":
		if args["path"] == "" {
			return "", fmt.Errorf("create_dir requires 'path' argument")
		}
		return f.CreateDir(args["path"])
	case "delete":
		if args["path"] == "" {
			return "", fmt.Errorf("delete requires 'path' argument")
		}
		return f.Delete(args["path"])
	case "move":
		if args["src"] == "" || args["dst"] == "" {
			return "", fmt.Errorf("move requires 'src' and 'dst' arguments")
		}
		return f.Move(args["src"], args["dst"])
	case "copy":
		if args["src"] == "" || args["dst"] == "" {
			return "", fmt.Errorf("copy requires 'src' and 'dst' arguments")
		}
		return f.Copy(args["src"], args["dst"])
	default:
		return "", fmt.Errorf("unknown file operation: %q", name)
	}
}
