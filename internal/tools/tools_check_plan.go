package tools

import (
	"fmt"

	"github.com/kapitanov/ucode/internal/iface"
)

func init() {
	register("check_plan", "check agent's plan's item as completed", checkPlanToolExecute)
}

type (
	checkPlanToolArgs struct {
		Index int `json:"index" jsonschema_description:"plan item index to check as completed"`
	}

	checkPlanToolResult struct {
		Items []checkPlanToolResultItem `json:"items" jsonschema_description:"plan items"`
	}

	checkPlanToolResultItem struct {
		Index int    `json:"index" jsonschema_description:"plan item index"`
		Done  bool   `json:"done"  jsonschema_description:"plan item completion status"`
		Title string `json:"title" jsonschema_description:"plan item title"`
	}
)

func checkPlanToolExecute(ctx iface.Context, args checkPlanToolArgs) (checkPlanToolResult, error) {
	result, err := checkPlanExecuteImpl(ctx, args)
	if err != nil {
		toolCall := checkPlanToolDescribe(args)
		ctx.ToolCall(toolCall).Failure(err.Error())
		return checkPlanToolResult{}, err
	}

	return result, nil
}

func checkPlanToolDescribe(args checkPlanToolArgs) iface.ToolCall {
	return iface.ToolCall{
		Type: "CHECK_PLAN",
		Args: fmt.Sprintf("#%d", args.Index),
	}
}

func checkPlanExecuteImpl(ctx iface.Context, args checkPlanToolArgs) (checkPlanToolResult, error) {
	plan := ctx.CheckPlanItem(args.Index)

	result := checkPlanToolResult{
		Items: make([]checkPlanToolResultItem, len(plan.Items)),
	}
	for i := range plan.Items {
		result.Items[i] = checkPlanToolResultItem{
			Index: plan.Items[i].Index,
			Done:  plan.Items[i].Done,
			Title: plan.Items[i].Title,
		}
	}

	return result, nil
}
