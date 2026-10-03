package tools

import (
	"github.com/kapitanov/ucode/internal/iface"
)

const RemoveFile Name = "rm_file"

func init() {
	register(RemoveFile, "remove a file", rmFileToolExecute)
}

type (
	rmFileToolArgs struct {
		Path string `json:"path" jsonschema_description:"path to the file to remove"`
	}

	rmFileToolResult struct {
		Removed bool `json:"removed" jsonschema_description:"whether the file was removed successfully"`
	}
)

func rmFileToolExecute(ctx iface.Context, args rmFileToolArgs) (rmFileToolResult, error) {
	toolCall := rmFileToolDescribe(args)

	callToken := ctx.ToolCall(toolCall)
	result, err := rmFileToolExecuteImpl(ctx, args)
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

func rmFileToolExecuteImpl(ctx iface.Context, args rmFileToolArgs) (rmFileToolResult, error) {
	err := ctx.Sandbox().RemoveFile(args.Path)
	if err != nil {
		return rmFileToolResult{}, err
	}

	return rmFileToolResult{Removed: true}, nil
}
