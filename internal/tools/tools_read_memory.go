package tools

import (
	"github.com/kapitanov/ucode/internal/iface"
)

func init() {
	register("read_memory", "read agent's memory", readMemoryToolExecute)
}

type (
	readMemoryToolArgs struct{}

	readMemoryToolResult struct {
		Items []readMemoryToolResultItem `json:"items" jsonschema_description:"memory items"`
	}

	readMemoryToolResultItem struct {
		Key   string `json:"key"   jsonschema_description:"memory item key"`
		Value string `json:"value" jsonschema_description:"memory item value"`
	}
)

func readMemoryToolExecute(ctx iface.Context, args readMemoryToolArgs) (readMemoryToolResult, error) {
	toolCall := readMemoryToolDescribe(args)

	callToken := ctx.ToolCall(toolCall)
	result, err := readMemoryExecuteImpl(ctx, args)
	if err != nil {
		callToken.Failure(err.Error())
		return readMemoryToolResult{}, err
	}
	callToken.Success()

	return result, nil
}

func readMemoryToolDescribe(_ readMemoryToolArgs) iface.ToolCall {
	return iface.ToolCall{
		Type: "READ_MEMORY",
		Args: "",
	}
}

func readMemoryExecuteImpl(ctx iface.Context, _ readMemoryToolArgs) (readMemoryToolResult, error) {
	memory := ctx.Memory()

	result := readMemoryToolResult{
		Items: make([]readMemoryToolResultItem, len(memory.Items)),
	}
	for i := range memory.Items {
		result.Items[i] = readMemoryToolResultItem{
			Key:   memory.Items[i].Key,
			Value: memory.Items[i].Value,
		}
	}

	return result, nil
}
