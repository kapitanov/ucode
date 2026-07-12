package tools

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tools/difftool"
	"github.com/kapitanov/ucode/internal/tools/guardrails"
)

func init() {
	register("edit_file", "edit a file (non-existing file will be created)", editFileToolExecute)
}

type (
	editFileToolArgs struct {
		Path string `json:"path" jsonschema_description:"path to the file to write"`
		Diff string `json:"diff" jsonschema_description:"unified diff to apply to the file, compatible with GNU patch. Format: '--- a/file\\n+++ b/file\\n@@ -L,S +L,S @@\\n context\\n-removed\\n+added'. Use context lines (no prefix) around changes. For new files, use /dev/null as the source path."`
	}

	editFileToolResult struct{}
)

func editFileToolExecute(ctx iface.Context, args editFileToolArgs) (editFileToolResult, error) {
	toolCall := editFileToolDescribe(args)

	callToken := ctx.ToolCall(toolCall)
	result, err := editFileToolExecuteImpl(args)
	if err != nil {
		callToken.Failure(err.Error())
		return editFileToolResult{}, err
	}
	callToken.Success()

	return result, nil
}

func editFileToolDescribe(args editFileToolArgs) iface.ToolCall {
	preview := difftool.Preview(args.Diff)

	return iface.ToolCall{
		Type: "EDIT",
		Args: fmt.Sprintf("%s (+%d -%d)\n%s", args.Path, preview.Added, preview.Removed, strings.Join(preview.Lines, "\n")),
	}
}

func editFileToolExecuteImpl(args editFileToolArgs) (editFileToolResult, error) {
	path, err := guardrails.NormalizePath(args.Path)
	if err != nil {
		return editFileToolResult{}, err
	}
	if !guardrails.IsAllowedPath(path) {
		return editFileToolResult{}, fmt.Errorf("access to path %q is not allowed", path)
	}

	original := ""
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return editFileToolResult{}, fmt.Errorf("failed to read file %q: %v", args.Path, err)
	} else if err == nil {
		original = string(existing)
	}

	patched, err := difftool.Apply(original, args.Diff)
	if err != nil {
		return editFileToolResult{}, fmt.Errorf("failed to apply diff to %q: %v", args.Path, err)
	}

	err = os.WriteFile(path, []byte(patched), 0666)
	if err != nil {
		return editFileToolResult{}, fmt.Errorf("failed to write file %q: %v", args.Path, err)
	}

	return editFileToolResult{}, nil
}
