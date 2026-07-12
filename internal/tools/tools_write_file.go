package tools

import (
	"fmt"
	"os"

	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tools/guardrails"
	"github.com/pyk/byten"
)

func init() {
	register("write_file", "write a file (existing file will be overwritten)", writeFileToolExecute)
}

type (
	writeFileToolArgs struct {
		Path    string `json:"path"    jsonschema_description:"path to the file to write"`
		Content string `json:"content" jsonschema_description:"content to write to the file"`
	}

	writeFileToolResult struct{}
)

func writeFileToolExecute(ctx iface.Context, args writeFileToolArgs) (writeFileToolResult, error) {
	toolCall := writeFileToolDescribe(args)

	callToken := ctx.ToolCall(toolCall)
	result, err := writeFileToolExecuteImpl(args)
	if err != nil {
		callToken.Failure(err.Error())
		return writeFileToolResult{}, err
	}
	callToken.Success()

	return result, nil
}

func writeFileToolDescribe(args writeFileToolArgs) iface.ToolCall {
	return iface.ToolCall{
		Type: "WRITE",
		Args: fmt.Sprintf("%s %s", args.Path, byten.Size(int64(len(args.Content)))),
	}
}

func writeFileToolExecuteImpl(args writeFileToolArgs) (writeFileToolResult, error) {
	path, err := guardrails.NormalizePath(args.Path)
	if err != nil {
		return writeFileToolResult{}, err
	}
	if !guardrails.IsAllowedPath(path) {
		return writeFileToolResult{}, fmt.Errorf("access to path %q is not allowed", path)
	}

	err = os.WriteFile(path, []byte(args.Content), 0666)
	if err != nil {
		return writeFileToolResult{}, fmt.Errorf("failed to write file %q: %v", args.Path, err)
	}

	return writeFileToolResult{}, nil
}
