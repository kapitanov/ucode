package tools

import (
	"fmt"

	"github.com/kapitanov/ucode/internal/iface"
)

const RunSubagent Name = "run_subagent"

func init() {
	register(RunSubagent, "run a subagent", runSubagentToolExecute)
}

type (
	runSubagentToolArgs struct {
		Agent   string `json:"agent"    jsonschema_description:"subagent name"`
		Request string `json:"request" jsonschema_description:"subagent request"`
	}

	runSubagentToolResult struct {
		Response string `json:"response" jsonschema_description:"subagent response"`
	}
)

func runSubagentToolExecute(ctx iface.Context, args runSubagentToolArgs) (runSubagentToolResult, error) {
	agent := ctx.Agents().Select(args.Agent)
	if agent == nil {
		return runSubagentToolResult{}, fmt.Errorf("no such agent: %q", args.Agent)
	}

	response, err := ctx.RunSubagent(agent, args.Request)
	if err != nil {
		ctx.ToolCall(iface.ToolCall{Type: "SUBAGENT", Args: args.Agent}).Failure(err.Error())
	}
	return runSubagentToolResult{Response: response}, err
}
