package tools

import (
	"fmt"
	"os"
)

func init() {
	defineTool("ls", "list files in a directory", lsTool, lsToolDescribe)
}

type (
	lsToolArgs struct {
		Dir string `json:"dir" description:"directory to list"`
	}

	lsToolResult struct {
		Dirs  []string `json:"dirs" description:"list of subdirectories in the directory"`
		Files []string `json:"files" description:"list of files in the directory"`
	}
)

func lsTool(args lsToolArgs) (lsToolResult, error) {
	dir, err := normalizePath(args.Dir)
	if err != nil {
		return lsToolResult{}, err
	}

	var result lsToolResult
	entries, err := os.ReadDir(dir)
	if err != nil {
		return result, fmt.Errorf("failed to read directory %q: %v", args.Dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			result.Dirs = append(result.Dirs, entry.Name())
		} else {
			result.Files = append(result.Files, entry.Name())
		}
	}

	return result, nil
}

func lsToolDescribe(args lsToolArgs) string {
	return fmt.Sprintf("LS %s", args.Dir)
}
