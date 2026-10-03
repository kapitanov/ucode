package tools

import (
	"github.com/kapitanov/ucode/internal/iface"
)

const GrepFiles Name = "grep_files"

func init() {
	register(GrepFiles, "search files in a directory", grepFilesToolExecute)
}

type (
	grepFilesToolArgs struct {
		Pattern       string  `json:"pattern"                  jsonschema_description:"The search pattern or regex to look for"`
		Path          *string `json:"path,omitempty"           jsonschema_description:"Optional path to search in (file or directory)"`
		FileType      *string `json:"file_type,omitempty"      jsonschema_description:"Optional file extension to limit search to (e.g., 'go', 'js', 'py')"`
		CaseSensitive *bool   `json:"case_sensitive,omitempty" jsonschema_description:"Whether the search should be case sensitive (default: false)"`
	}

	grepFilesToolResult struct {
		Results    []string `json:"results"     jsonschema_description:"list of search results"`
		TotalCount int      `json:"total_count" jsonschema_description:"total number of matches found"`
	}
)

func grepFilesToolExecute(ctx iface.Context, args grepFilesToolArgs) (grepFilesToolResult, error) {
	toolCall := grepFilesToolDescribe(args)

	callToken := ctx.ToolCall(toolCall)
	result, err := grepFilesToolExecuteImpl(ctx, args)
	if err != nil {
		callToken.Failure(err.Error())
		return grepFilesToolResult{}, err
	}
	callToken.Success()

	return result, nil
}

func grepFilesToolDescribe(args grepFilesToolArgs) iface.ToolCall {
	argsText := args.Pattern

	if args.Path != nil && *args.Path != "" {
		argsText += " " + *args.Path
	}

	if args.FileType != nil && *args.FileType != "" {
		argsText += " --type=" + *args.FileType
	}

	if args.CaseSensitive != nil && *args.CaseSensitive {
		argsText += " --case-sensitive"
	}

	return iface.ToolCall{
		Type: "GREP",
		Args: argsText,
	}
}

func grepFilesToolExecuteImpl(ctx iface.Context, args grepFilesToolArgs) (grepFilesToolResult, error) {
	results, err := ctx.Sandbox().SearchFiles(args.Pattern, args.Path, args.FileType, args.CaseSensitive)
	if err != nil {
		return grepFilesToolResult{}, err
	}
	result := grepFilesToolResult{
		Results:    results,
		TotalCount: len(results),
	}

	// Limit output to prevent overwhelming responses
	const maxResults = 50
	if len(result.Results) > maxResults {
		result.Results = result.Results[:maxResults]
	}

	return result, nil
}
