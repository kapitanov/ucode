package tools

import (
	"fmt"
	"os"
)

const maxFileSize = 10 * 1024 * 1024 // 10 MB

func init() {
	defineTool("read", "read a file", readTool, readToolDescribe)
}

type (
	readToolArgs struct {
		File string `json:"file" description:"file to read"`
	}

	readToolResult struct {
		Text string `json:"text" description:"content of the file"`
	}
)

func readTool(args readToolArgs) (readToolResult, error) {
	path, err := normalizePath(args.File)
	if err != nil {
		return readToolResult{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return readToolResult{}, fmt.Errorf("failed to stat file %q: %v", args.File, err)
	}

	if info.Size() > maxFileSize {
		return readToolResult{}, fmt.Errorf("file %q is too large (%d bytes, max %d bytes)", args.File, info.Size(), maxFileSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return readToolResult{}, fmt.Errorf("failed to read file %q: %v", args.File, err)
	}

	return readToolResult{Text: string(data)}, nil
}

func readToolDescribe(args readToolArgs) string {
	return fmt.Sprintf("READ %s", args.File)
}
