package tools

import (
	"fmt"
	"os"

	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tools/guardrails"
)

func init() {
	register("rm_file", "remove a file", rmFileToolExecute)
}

type (
	rmFileToolArgs struct {
		Path string `json:"path" jsonschema_description:"path to the file to remove"`
	}

	rmFileToolResult struct{}
)

func rmFileToolExecute(ctx iface.Context, args rmFileToolArgs) (rmFileToolResult, error) {
	toolCall := rmFileToolDescribe(args)

	callToken := ctx.ToolCall(toolCall)
	result, err := rmFileToolExecuteImpl(args)
	if err != nil {
		callToken.Failure(err.Error())
		return rmFileToolResult{}, err
	}
	callToken.Success()

	return result, nil
}

func rmFileToolDescribe(args rmFileToolArgs) iface.ToolCall {
	return iface.ToolCall{
		Type: "RM",
		Args: args.Path,
	}
}

func rmFileToolExecuteImpl(args rmFileToolArgs) (rmFileToolResult, error) {
	path, err := guardrails.NormalizePath(args.Path)
	if err != nil {
		return rmFileToolResult{}, err
	}
	if !guardrails.IsAllowedPath(path) {
		return rmFileToolResult{}, fmt.Errorf("access to path %q is not allowed", path)
	}

	err = os.Remove(path)
	if err != nil {
		return rmFileToolResult{}, fmt.Errorf("failed to remove file %q: %v", args.Path, err)
	}

	return rmFileToolResult{}, nil
}
