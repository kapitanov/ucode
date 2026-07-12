package tools

import (
	"fmt"
	"os"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tools/guardrails"
)

func init() {
	register("list_files", "list files in a directory", listFilesToolExecute)
}

type (
	listFilesToolArgs struct {
		Dir     string  `json:"dir"               jsonschema_description:"directory to list"`
		Pattern *string `json:"pattern,omitempty" jsonschema_description:"pattern to match files (glob)"`
	}

	listFilesToolResult struct {
		Dirs  []string `json:"dirs"  jsonschema_description:"list of subdirectories in the directory"`
		Files []string `json:"files" jsonschema_description:"list of files in the directory"`
	}
)

func listFilesToolExecute(ctx iface.Context, args listFilesToolArgs) (listFilesToolResult, error) {
	toolCall := listFilesToolDescribe(args)

	callToken := ctx.ToolCall(toolCall)
	result, err := listFilesToolExecuteImpl(args)
	if err != nil {
		callToken.Failure(err.Error())
		return listFilesToolResult{}, err
	}
	callToken.Success()

	return result, nil
}

func listFilesToolDescribe(args listFilesToolArgs) iface.ToolCall {
	argsText := args.Dir
	if args.Pattern != nil {
		argsText += " " + *args.Pattern
	}

	return iface.ToolCall{
		Type: "LS",
		Args: argsText,
	}
}

func listFilesToolExecuteImpl(args listFilesToolArgs) (listFilesToolResult, error) {
	dir, err := guardrails.NormalizePath(args.Dir)
	if err != nil {
		return listFilesToolResult{}, err
	}

	args.Dir = dir

	filter := func(string) bool { return true }
	if args.Pattern != nil {
		filter = func(path string) bool {
			match, err := doublestar.Match(*args.Pattern, path)
			if err != nil {
				return false
			}
			return match
		}
	}

	entries, err := os.ReadDir(args.Dir)
	if err != nil {
		return listFilesToolResult{}, fmt.Errorf("failed to read directory %q: %v", args.Dir, err)
	}

	var result listFilesToolResult
	for _, entry := range entries {
		if !guardrails.IsAllowedPath(entry.Name()) {
			continue
		}

		if !filter(entry.Name()) {
			continue
		}

		if entry.IsDir() {
			result.Dirs = append(result.Dirs, entry.Name())
		} else {
			result.Files = append(result.Files, entry.Name())
		}
	}

	return result, nil
}
