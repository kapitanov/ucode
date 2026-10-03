package tools

import (
	"fmt"
	"strings"

	"github.com/kapitanov/ucode/internal/iface"
	"github.com/pyk/byten"
)

const WriteMemory Name = "write_memory"

func init() {
	register(WriteMemory, "write agent's memory", writeMemoryToolExecute)
}

type (
	writeMemoryToolArgs struct {
		Items []writeMemoryToolResultItem `json:"items" jsonschema_description:"memory items to write"`
	}

	writeMemoryToolResult struct {
		Keys []string `json:"items" jsonschema_description:"memory item keys"`
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
	var keys []string
	totalSize := 0
	for _, item := range args.Items {
		keys = append(keys, item.Key)
		totalSize += len(item.Value)
	}

	return iface.ToolCall{
		Type: "MEMORY:WRITE",
		Args: fmt.Sprintf("%s %s", strings.Join(keys, ","), byten.Size(int64(totalSize))),
	}
}

func writeMemoryExecuteImpl(ctx iface.Context, args writeMemoryToolArgs) (writeMemoryToolResult, error) {
	memory := ctx.Memory()
	for _, item := range args.Items {
		memory = ctx.WriteMemory(item.Key, item.Value)
	}

	result := writeMemoryToolResult{
		Keys: make([]string, len(memory.Items)),
	}
	for i := range memory.Items {
		result.Keys[i] = memory.Items[i].Key
	}

	return result, nil
}
