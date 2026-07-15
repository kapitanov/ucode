package tools

import (
	"fmt"

	"github.com/kapitanov/ucode/internal/iface"
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
	result, err := writeFileToolExecuteImpl(ctx, args)
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

func writeFileToolExecuteImpl(ctx iface.Context, args writeFileToolArgs) (writeFileToolResult, error) {
	err := ctx.Sandbox().WriteFile(args.Path, []byte(args.Content))
	return writeFileToolResult{}, err
}
