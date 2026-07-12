package tools

import (
	"fmt"

	"github.com/kapitanov/ucode/internal/iface"
	"github.com/pyk/byten"
)

func init() {
	register("write_memory", "write agent's memory", writeMemoryToolExecute)
}

type (
	writeMemoryToolArgs struct {
		Key   string `json:"key"   jsonschema_description:"memory item key"`
		Value string `json:"value" jsonschema_description:"memory item value"`
	}

	writeMemoryToolResult struct {
		Items []writeMemoryToolResultItem `json:"items" jsonschema_description:"memory items"`
	}

	writeMemoryToolResultItem struct {
		Key   string `json:"key"   jsonschema_description:"memory item key"`
		Value string `json:"value" jsonschema_description:"memory item value"`
	}
)

func writeMemoryToolExecute(ctx iface.Context, args writeMemoryToolArgs) (writeMemoryToolResult, error) {
	toolCall := writeMemoryToolDescribe(args)

	callToken := ctx.ToolCall(toolCall)
	result, err := writeMemoryExecuteImpl(ctx, args)
	if err != nil {
		callToken.Failure(err.Error())
		return writeMemoryToolResult{}, err
	}
	callToken.Success()

	return result, nil
}

func writeMemoryToolDescribe(args writeMemoryToolArgs) iface.ToolCall {
	return iface.ToolCall{
		Type: "WRITE_MEMORY",
		Args: fmt.Sprintf("%s %s", args.Key, byten.Size(int64(len(args.Value)))),
	}
}

func writeMemoryExecuteImpl(ctx iface.Context, args writeMemoryToolArgs) (writeMemoryToolResult, error) {
	memory := ctx.WriteMemory(args.Key, args.Value)

	result := writeMemoryToolResult{
		Items: make([]writeMemoryToolResultItem, len(memory.Items)),
	}
	for i := range memory.Items {
		result.Items[i] = writeMemoryToolResultItem{
			Key:   memory.Items[i].Key,
			Value: memory.Items[i].Value,
		}
	}

	return result, nil
}
