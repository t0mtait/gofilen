# gofilen

CLI tool for basic file management within a user-specified directory.

## Usage

```bash
gofilen -dir <directory> <command> [args]
```

## Commands

- `list` — list entries in the target directory
- `info <path>` — show info for a file or directory
- `read <path>` — print file contents
- `write <path> -content <text>` — write text to a file (creates or overwrites)
- `delete <path>` — delete a file
- `mkdir <path>` — create a directory (including parents)
- `touch <path>` — create an empty file if it doesn’t exist
- `copy <src> <dst>` — copy a file
- `move <src> <dst>` — move or rename a file

All paths are resolved relative to `-dir` and are blocked from escaping that directory.

## Examples

```bash
gofilen -dir ./data list
gofilen -dir ./data info notes.txt
gofilen -dir ./data read notes.txt
gofilen -dir ./data write notes.txt -content "hello"
gofilen -dir ./data mkdir reports
gofilen -dir ./data copy notes.txt reports/notes.txt
```connect to filen network mount
language model trained on all data in filen mount
query data, update file names, move files around, create dirs, delete files, etc.

