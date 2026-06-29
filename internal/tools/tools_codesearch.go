package tools

import (
	"fmt"
	"os/exec"
	"strings"
)

func init() {
	defineTool("codesearch", "search files in a directory", codeSearchTool, codeSearchToolDescribe)
}

type (
	codeSearchToolArgs struct {
		Pattern       string `json:"pattern" jsonschema_description:"The search pattern or regex to look for"`
		Path          string `json:"path,omitempty" jsonschema_description:"Optional path to search in (file or directory)"`
		FileType      string `json:"file_type,omitempty" jsonschema_description:"Optional file extension to limit search to (e.g., 'go', 'js', 'py')"`
		CaseSensitive bool   `json:"case_sensitive,omitempty" jsonschema_description:"Whether the search should be case sensitive (default: false)"`
	}

	codeSearchToolResult struct {
		Result string `json:"result" description:"search results"`
	}
)

func codeSearchTool(args codeSearchToolArgs) (codeSearchToolResult, error) {
	if args.Path != "" {
		var err error
		args.Path, err = normalizePath(args.Path)
		if err != nil {
			return codeSearchToolResult{}, err
		}
	}

	// Build ripgrep command
	rgArgs := []string{"--line-number", "--with-filename", "--color=never"}

	// Add case sensitivity flag
	if !args.CaseSensitive {
		rgArgs = append(rgArgs, "--ignore-case")
	}

	// Add file type filter if specified
	if args.FileType != "" {
		rgArgs = append(rgArgs, "--type", args.FileType)
	}

	// Add pattern
	rgArgs = append(rgArgs, args.Pattern)

	// Add path if specified
	if args.Path != "" {
		rgArgs = append(rgArgs, args.Path)
	} else {
		rgArgs = append(rgArgs, ".")
	}

	cmd := exec.Command("rg", rgArgs...)
	output, err := cmd.Output()

	// ripgrep returns exit code 1 when no matches are found, which is not an error
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return codeSearchToolResult{}, nil
		}
		return codeSearchToolResult{}, fmt.Errorf("search failed: %w", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")

	// Limit output to prevent overwhelming responses
	if len(lines) > 50 {
		result = strings.Join(lines[:50], "\n") + fmt.Sprintf("\n... (showing first 50 of %d matches)", len(lines))
	}

	return codeSearchToolResult{Result: result}, nil
}

func codeSearchToolDescribe(args codeSearchToolArgs) string {
	description := "CODESEARCH "

	if !args.CaseSensitive {
		description += "(ignore-case) "
	}

	if args.FileType != "" {
		description += fmt.Sprintf("(file-type: %s) ", args.FileType)
	}

	description += args.Pattern
	description += " "

	if args.Path != "" {
		description += args.Path
	} else {
		description += "."
	}
	return description
}
