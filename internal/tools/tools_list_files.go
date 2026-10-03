package tools

import (
	"github.com/bmatcuk/doublestar/v4"
	"github.com/kapitanov/ucode/internal/iface"
)

const ListFiles Name = "list_files"

func init() {
	register(ListFiles, "list files in a directory", listFilesToolExecute)
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
	result, err := listFilesToolExecuteImpl(ctx, args)
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

func listFilesToolExecuteImpl(ctx iface.Context, args listFilesToolArgs) (listFilesToolResult, error) {
	dirs, files, err := ctx.Sandbox().ListFiles(args.Dir)
	if err != nil {
		return listFilesToolResult{}, err
	}

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

	result := listFilesToolResult{
		Dirs:  []string{},
		Files: []string{},
	}

	for _, dir := range dirs {
		if !filter(dir) {
			continue
		}

		result.Dirs = append(result.Dirs, dir)
	}

	for _, file := range files {
		if !filter(file) {
			continue
		}

		result.Files = append(result.Files, file)
	}

	return result, nil
}
