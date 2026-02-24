package main

import (
    "bufio"
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
)

const baseDir = "./data"

func main() {
    baseDirAbs, err := filepath.Abs(baseDir)
    if err != nil {
        fmt.Fprintln(os.Stderr, "invalid baseDir:", err)
        os.Exit(2)
    }

    reader := bufio.NewReader(os.Stdin)
    for {
        fmt.Println("\nGofilen - File Manager")
        fmt.Println("Base directory:", baseDirAbs)
        fmt.Println("1) List files")
        fmt.Println("2) File info")
        fmt.Println("3) Read file")
        fmt.Println("4) Write file")
        fmt.Println("5) Delete file")
        fmt.Println("6) Make directory")
        fmt.Println("7) Touch file")
        fmt.Println("8) Copy file")
        fmt.Println("9) Move file")
        fmt.Println("0) Exit")
        fmt.Print("Choose an option: ")

        choice, err := readLine(reader)
        if err != nil {
            exitErr(err)
        }

        switch choice {
        case "1":
            if err := listFiles(baseDirAbs); err != nil {
                exitErr(err)
            }
        case "2":
            path := mustPrompt(reader, "Enter path: ")
            target, err := secureJoin(baseDirAbs, path)
            if err != nil {
                exitErr(err)
            }
            if err := fileInfo(target); err != nil {
                exitErr(err)
            }
        case "3":
            path := mustPrompt(reader, "Enter path: ")
            target, err := secureJoin(baseDirAbs, path)
            if err != nil {
                exitErr(err)
            }
            if err := readFile(target); err != nil {
                exitErr(err)
            }
        case "4":
            path := mustPrompt(reader, "Enter path: ")
            content := mustPrompt(reader, "Enter content: ")
            target, err := secureJoin(baseDirAbs, path)
            if err != nil {
                exitErr(err)
            }
            if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
                exitErr(err)
            }
        case "5":
            path := mustPrompt(reader, "Enter path: ")
            target, err := secureJoin(baseDirAbs, path)
            if err != nil {
                exitErr(err)
            }
            if err := os.Remove(target); err != nil {
                exitErr(err)
            }
        case "6":
            path := mustPrompt(reader, "Enter directory path: ")
            target, err := secureJoin(baseDirAbs, path)
            if err != nil {
                exitErr(err)
            }
            if err := os.MkdirAll(target, 0o755); err != nil {
                exitErr(err)
            }
        case "7":
            path := mustPrompt(reader, "Enter path: ")
            target, err := secureJoin(baseDirAbs, path)
            if err != nil {
                exitErr(err)
            }
            if err := touchFile(target); err != nil {
                exitErr(err)
            }
        case "8":
            srcPath := mustPrompt(reader, "Enter source path: ")
            dstPath := mustPrompt(reader, "Enter destination path: ")
            src, err := secureJoin(baseDirAbs, srcPath)
            if err != nil {
                exitErr(err)
            }
            dst, err := secureJoin(baseDirAbs, dstPath)
            if err != nil {
                exitErr(err)
            }
            if err := copyFile(src, dst); err != nil {
                exitErr(err)
            }
        case "9":
            srcPath := mustPrompt(reader, "Enter source path: ")
            dstPath := mustPrompt(reader, "Enter destination path: ")
            src, err := secureJoin(baseDirAbs, srcPath)
            if err != nil {
                exitErr(err)
            }
            dst, err := secureJoin(baseDirAbs, dstPath)
            if err != nil {
                exitErr(err)
            }
            if err := os.Rename(src, dst); err != nil {
                exitErr(err)
            }
        case "0":
            fmt.Println("Goodbye.")
            return
        default:
            fmt.Println("Invalid option. Try again.")
        }
    }
}

func readLine(reader *bufio.Reader) (string, error) {
    line, err := reader.ReadString('\n')
    if err != nil && !errors.Is(err, io.EOF) {
        return "", err
    }
    return strings.TrimSpace(line), nil
}

func mustPrompt(reader *bufio.Reader, prompt string) string {
    for {
        fmt.Print(prompt)
        value, err := readLine(reader)
        if err != nil {
            exitErr(err)
        }
        if value != "" {
            return value
        }
        fmt.Println("Value cannot be empty.")
    }
}

func exitErr(err error) {
    fmt.Fprintln(os.Stderr, "Error:", err)
    os.Exit(1)
}

func secureJoin(baseDir, rel string) (string, error) {
    if rel == "" {
        return "", errors.New("path is empty")
    }
    baseAbs, err := filepath.Abs(baseDir)
    if err != nil {
        return "", err
    }
    targetAbs, err := filepath.Abs(filepath.Join(baseAbs, rel))
    if err != nil {
        return "", err
    }
    if targetAbs != baseAbs && !strings.HasPrefix(targetAbs, baseAbs+string(os.PathSeparator)) {
        return "", errors.New("path escapes base directory")
    }
    return targetAbs, nil
}

func listFiles(dir string) error {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return err
    }
    for _, entry := range entries {
        name := entry.Name()
        if entry.IsDir() {
            name += string(os.PathSeparator)
        }
        fmt.Println(name)
    }
    return nil
}

func fileInfo(path string) error {
    info, err := os.Stat(path)
    if err != nil {
        return err
    }
    fmt.Printf("Name: %s\n", info.Name())
    fmt.Printf("Size: %d bytes\n", info.Size())
    fmt.Printf("Mode: %s\n", info.Mode())
    fmt.Printf("Modified: %s\n", info.ModTime().Format(timeLayout))
    fmt.Printf("Directory: %t\n", info.IsDir())
    return nil
}

const timeLayout = "2006-01-02 15:04:05 -0700"

func readFile(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    fmt.Print(string(data))
    return nil
}

func touchFile(path string) error {
    file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
    if err != nil {
        return err
    }
    return file.Close()
}

func copyFile(src, dst string) error {
    srcFile, err := os.Open(src)
    if err != nil {
        return err
    }
    defer srcFile.Close()

    info, err := srcFile.Stat()
    if err != nil {
        return err
    }
    if info.IsDir() {
        return errors.New("copy source is a directory")
    }

    dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
    if err != nil {
        return err
    }
    defer dstFile.Close()

    if _, err := io.Copy(dstFile, srcFile); err != nil {
        return err
    }
    return nil
}

