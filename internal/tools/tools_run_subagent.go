package tools

import (
	"fmt"

	"github.com/kapitanov/ucode/internal/iface"
)

func init() {
	register("run_subagent", "run a subagent", runSubagentToolExecute)
}

type (
	runSubagentToolArgs struct {
		Role    string `json:"role"    jsonschema_description:"subagent role, any of: 'reviewer', 'coder', 'tester'"`
		Request string `json:"request" jsonschema_description:"subagent request"`
	}

	runSubagentToolResult struct {
		Response string `json:"response" jsonschema_description:"subagent response"`
	}
)

func runSubagentToolExecute(ctx iface.Context, args runSubagentToolArgs) (runSubagentToolResult, error) {
	agent := ctx.Agents().ByRole(args.Role)
	if agent == nil {
		return runSubagentToolResult{}, fmt.Errorf("no such agent: %q", args.Role)
	}

	response, err := ctx.RunSubagent(agent, args.Request)
	if err != nil {
		ctx.ToolCall(iface.ToolCall{Type: "SUBAGENT", Args: args.Role}).Failure(err.Error())
	}
	return runSubagentToolResult{Response: response}, err
}
