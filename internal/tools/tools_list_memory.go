package tools

import (
	"github.com/kapitanov/ucode/internal/iface"
)

const ListMemory Name = "list_memory"

func init() {
	register(ListMemory, "list agent's memory keys", listMemoryToolExecute)
}

type (
	listMemoryToolArgs struct{}

	listMemoryToolResult struct {
		Keys []string `json:"items" jsonschema_description:"memory item keys"`
	}
)

func listMemoryToolExecute(ctx iface.Context, args listMemoryToolArgs) (listMemoryToolResult, error) {
	toolCall := listMemoryToolDescribe(args)

	callToken := ctx.ToolCall(toolCall)
	result, err := listMemoryExecuteImpl(ctx, args)
	if err != nil {
		callToken.Failure(err.Error())
		return listMemoryToolResult{}, err
	}
	callToken.Success()

	return result, nil
}

func listMemoryToolDescribe(_ listMemoryToolArgs) iface.ToolCall {
	return iface.ToolCall{
		Type: "MEMORY:LS",
		Args: "",
	}
}

func listMemoryExecuteImpl(ctx iface.Context, _ listMemoryToolArgs) (listMemoryToolResult, error) {
	memory := ctx.Memory()

	result := listMemoryToolResult{
		Keys: make([]string, len(memory.Items)),
	}
	for i := range memory.Items {
		result.Keys[i] = memory.Items[i].Key
	}

	return result, nil
}
