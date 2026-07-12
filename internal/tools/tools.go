package tools

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kapitanov/ucode/internal/iface"
	"github.com/revrost/go-openrouter"
	"github.com/revrost/go-openrouter/jsonschema"
)

type Tool struct {
	Name       string
	Definition openrouter.Tool
	Execute    func(ctx iface.Context, args string) ToolResult
}

type ToolResult struct {
	Result string
	Error  error
}

func (r ToolResult) String() string {
	if r.Error != nil {
		return fmt.Sprintf("Error: %v", r.Error)
	}
	return r.Result
}

var (
	tools = make(map[string]*Tool)
)

func All() []*Tool {
	all := slices.Collect(maps.Values(tools))
	slices.SortFunc(all, func(a, b *Tool) int {
		return strings.Compare(a.Name, b.Name)
	})
	return all
}

func Execute(ctx iface.Context, toolName string, toolArgs string) ToolResult {
	tool, ok := tools[toolName]
	if !ok {
		return ToolResult{Error: fmt.Errorf("tool %q not found", toolName)}
	}

	return tool.Execute(ctx, toolArgs)
}

func register[T, R any](
	name, description string,
	execute func(ctx iface.Context, args T) (R, error),
) {
	schema, err := jsonschema.GenerateSchema[T]()
	if err != nil {
		panic(err)
	}

	tool := &Tool{
		Name: name,
		Definition: openrouter.Tool{
			Type: openrouter.ToolTypeFunction,
			Function: &openrouter.FunctionDefinition{
				Name:        name,
				Description: description,
				Strict:      true,
				Parameters:  schema,
			},
		},
		Execute: func(ctx iface.Context, rawArgs string) ToolResult {
			var args T
			if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
				return ToolResult{Error: fmt.Errorf("invalid arguments for tool %q: %v", name, err)}
			}

			v, err := execute(ctx, args)
			if err != nil {
				return ToolResult{Error: err}
			}

			result, err := json.Marshal(v)
			if err != nil {
				return ToolResult{Error: err}
			}

			return ToolResult{Result: string(result)}
		},
	}
	tools[name] = tool
}
