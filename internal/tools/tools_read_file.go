package tools

import (
	"fmt"

	"github.com/kapitanov/ucode/internal/iface"
)

const ReadFile Name = "read_file"

func init() {
	register(ReadFile, "read a file", readFileToolExecute)
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
	result, err := readFileToolExecuteImpl(ctx, args)
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

func readFileToolExecuteImpl(ctx iface.Context, args readFileToolArgs) (readFileToolResult, error) {
	data, err := ctx.Sandbox().ReadFile(args.Path)
	if err != nil {
		return readFileToolResult{}, fmt.Errorf("failed to read file %q: %v", args.Path, err)
	}

	return readFileToolResult{Content: string(data)}, nil
}
