package tools

import (
	"fmt"

	"github.com/kapitanov/ucode/internal/etc/difftool"
	"github.com/kapitanov/ucode/internal/iface"
)

func init() {
	register("edit_file", "edit a file (non-existing file will be created)", editFileToolExecute)
}

type (
	editFileToolArgs struct {
		Path string `json:"path" jsonschema_description:"path to the file to write"`
		Diff string `json:"diff" jsonschema_description:"unified diff to apply to the file, compatible with GNU patch. Format: '--- a/file\\n+++ b/file\\n@@ -L,S +L,S @@\\n context\\n-removed\\n+added'. Use context lines (no prefix) around changes. For new files, use /dev/null as the source path."`
	}

	editFileToolResult struct {
		Content string `json:"content" jsonschema_description:"patched content of the file"`
	}
)

func editFileToolExecute(ctx iface.Context, args editFileToolArgs) (editFileToolResult, error) {
	toolCall := editFileToolDescribe(args)

	callToken := ctx.ToolCall(toolCall)
	result, err := editFileToolExecuteImpl(ctx, args)
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
		Args: fmt.Sprintf("%s (+%d -%d)", args.Path, preview.Added, preview.Removed),
	}
}

func editFileToolExecuteImpl(ctx iface.Context, args editFileToolArgs) (editFileToolResult, error) {
	patched, err := ctx.Sandbox().PatchFile(args.Path, args.Diff)
	if err != nil {
		return editFileToolResult{}, err
	}

	return editFileToolResult{Content: string(patched)}, nil
}
