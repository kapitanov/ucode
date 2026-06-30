package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	const description = `Make edits to a text file.

Replaces 'old_str' with 'new_str' in the given file. 'old_str' and 'new_str' MUST be different from each other.

If the file specified with path doesn't exist, it will be created.
`
	defineTool("edit", description, editTool, editToolDescribe)
}

type (
	editToolArgs struct {
		File   string `json:"file" description:"file to edit"`
		OldStr string `json:"old_str" description:"string to be replaced"`
		NewStr string `json:"new_str" description:"string to replace with"`
	}

	editToolResult struct {
		Text string `json:"text" description:"content of the file"`
	}
)

const maxEditFileSize = 10 * 1024 * 1024 // 10 MB

func editTool(args editToolArgs) (editToolResult, error) {
	if args.OldStr == args.NewStr {
		return editToolResult{}, fmt.Errorf("old_str and new_str must be different")
	}

	path, err := normalizePath(args.File)
	if err != nil {
		return editToolResult{}, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return editToolResult{}, fmt.Errorf("failed to create directories for file %q: %v", args.File, err)
	}

	info, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return editToolResult{}, fmt.Errorf("failed to stat file %q: %v", args.File, err)
	}
	if err == nil && info.Size() > maxEditFileSize {
		return editToolResult{}, fmt.Errorf("file %q is too large (%d bytes, max %d bytes)", args.File, info.Size(), maxEditFileSize)
	}

	bs, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return editToolResult{}, fmt.Errorf("failed to read file %q: %v", args.File, err)
	}

	oldContent := string(bs)
	var newContent string
	if args.OldStr == "" {
		if oldContent != "" {
			return editToolResult{}, fmt.Errorf("old_str is empty but file already has content; use exact string to replace")
		}
		newContent = args.NewStr
	} else {
		// Count occurrences first to ensure we have exactly one match
		count := strings.Count(oldContent, args.OldStr)
		if count == 0 {
			return editToolResult{}, fmt.Errorf("old_str not found in file")
		}
		if count > 1 {
			return editToolResult{}, fmt.Errorf("old_str found %d times in file, must be unique", count)
		}

		newContent = strings.Replace(oldContent, args.OldStr, args.NewStr, 1)
	}

	err = os.WriteFile(path, []byte(newContent), 0644)
	if err != nil {
		return editToolResult{}, fmt.Errorf("failed to write file %q: %v", args.File, err)
	}

	return editToolResult{Text: "OK"}, nil
}

func editToolDescribe(args editToolArgs) string {
	return fmt.Sprintf("EDIT %s", args.File)
}
