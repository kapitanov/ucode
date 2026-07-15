package tools

import (
	"github.com/kapitanov/ucode/internal/iface"
)

func init() {
	register("clear_plan", "clear agent's plan", clearPlanToolExecute)
}

type (
	clearPlanToolArgs   struct{}
	clearPlanToolResult struct{}
)

func clearPlanToolExecute(ctx iface.Context, args clearPlanToolArgs) (clearPlanToolResult, error) {
	result, err := clearPlanExecuteImpl(ctx, args)
	if err != nil {
		toolCall := clearPlanToolDescribe(args)
		ctx.ToolCall(toolCall).Failure(err.Error())
		return clearPlanToolResult{}, err
	}

	ctx.UI().PrintPlan()
	return result, nil
}

func clearPlanToolDescribe(_ clearPlanToolArgs) iface.ToolCall {
	return iface.ToolCall{
		Type: "PLAN:CLEAR",
		Args: "",
	}
}

func clearPlanExecuteImpl(ctx iface.Context, _ clearPlanToolArgs) (clearPlanToolResult, error) {
	ctx.ClearPlan()
	return clearPlanToolResult{}, nil
}
