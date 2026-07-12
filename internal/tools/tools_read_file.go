package tools

import (
	"fmt"
	"os"

	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tools/guardrails"
)

func init() {
	register("read_file", "read a file", readFileToolExecute)
}

type (
	readFileToolArgs struct {
		Path string `json:"path" jsonschema_description:"path to the file to read"`
	}

	readFileToolResult struct {
		Content string `json:"content" jsonschema_description:"content of the file"`
	}
)

func readFileToolExecute(ctx iface.Context, args readFileToolArgs) (readFileToolResult, error) {
	toolCall := readFileToolDescribe(args)

	callToken := ctx.ToolCall(toolCall)
	result, err := readFileToolExecuteImpl(args)
	if err != nil {
		callToken.Failure(err.Error())
		return readFileToolResult{}, err
	}
	callToken.Success()

	return result, nil
}

func readFileToolDescribe(args readFileToolArgs) iface.ToolCall {
	return iface.ToolCall{
		Type: "READ",
		Args: args.Path,
	}
}

func readFileToolExecuteImpl(args readFileToolArgs) (readFileToolResult, error) {
	path, err := guardrails.NormalizePath(args.Path)
	if err != nil {
		return readFileToolResult{}, err
	}
	if !guardrails.IsAllowedPath(path) {
		return readFileToolResult{}, fmt.Errorf("access to path %q is not allowed", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return readFileToolResult{}, fmt.Errorf("failed to read file %q: %v", args.Path, err)
	}

	return readFileToolResult{Content: string(data)}, nil
}
