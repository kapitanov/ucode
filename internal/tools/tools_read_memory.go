package tools

import (
	"strings"

	"github.com/kapitanov/ucode/internal/iface"
)

const ReadMemory Name = "read_memory"

func init() {
	register(ReadMemory, "read agent's memory", readMemoryToolExecute)
}

type (
	readMemoryToolArgs struct {
		Keys []string `json:"keys" jsonschema_description:"memory item keys"`
	}

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

func readMemoryToolDescribe(args readMemoryToolArgs) iface.ToolCall {
	return iface.ToolCall{
		Type: "MEMORY:READ",
		Args: strings.Join(args.Keys, ","),
	}
}

func readMemoryExecuteImpl(ctx iface.Context, args readMemoryToolArgs) (readMemoryToolResult, error) {
	memory := ctx.Memory()

	result := readMemoryToolResult{
		Items: []readMemoryToolResultItem{},
	}
	for _, key := range args.Keys {
		value := memory.Get(key)
		if value != "" {
			result.Items = append(result.Items, readMemoryToolResultItem{
				Key:   key,
				Value: value,
			})
		}
	}

	return result, nil
}
