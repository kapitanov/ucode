package tools

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tools/guardrails"
)

func init() {
	if _, err := exec.LookPath("rg"); err != nil {
		panic("ripgrep (rg) is not installed or not in PATH")
	}

	register("grep_files", "search files in a directory", grepFilesToolExecute)
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
	result, err := grepFilesToolExecuteImpl(args)
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

func grepFilesToolExecuteImpl(args grepFilesToolArgs) (grepFilesToolResult, error) {
	if args.Path == nil || *args.Path == "" {
		args.Path = new(".")
	}

	path, err := guardrails.NormalizePath(*args.Path)
	if err != nil {
		return grepFilesToolResult{}, err
	}
	if !guardrails.IsAllowedPath(path) {
		return grepFilesToolResult{}, fmt.Errorf("access to path %q is not allowed", path)
	}

	// Build ripgrep command
	rgArgs := []string{"--line-number", "--with-filename", "--color=never"}

	// Add case sensitivity flag
	if args.CaseSensitive == nil || !*args.CaseSensitive {
		rgArgs = append(rgArgs, "--ignore-case")
	}

	// Add file type filter if specified
	if args.FileType != nil && *args.FileType != "" {
		rgArgs = append(rgArgs, "--type", *args.FileType)
	}

	rgArgs = append(rgArgs, args.Pattern, path)

	cmd := exec.Command("rg", rgArgs...)
	output, err := cmd.Output()

	// ripgrep returns exit code 1 when no matches are found, which is not an error
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return grepFilesToolResult{}, nil
		}
		return grepFilesToolResult{}, fmt.Errorf("search failed: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return grepFilesToolResult{Results: []string{}, TotalCount: 0}, nil
	}

	lines := strings.Split(outputStr, "\n")

	result := grepFilesToolResult{
		Results:    lines,
		TotalCount: len(lines),
	}

	// Limit output to prevent overwhelming responses
	const maxResults = 50
	if len(result.Results) > maxResults {
		result.Results = result.Results[:maxResults]
	}

	return result, nil
}
