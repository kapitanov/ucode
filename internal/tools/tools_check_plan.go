package tools

import (
	"fmt"
	"strings"

	"github.com/kapitanov/ucode/internal/iface"
)

func init() {
	register("check_plan", "check agent's plan's item as completed", checkPlanToolExecute)
}

type (
	checkPlanToolArgs struct {
		Indices []int `json:"indices" jsonschema_description:"plan item indices to check as completed"`
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
	toolCall := checkPlanToolDescribe(args)

	plan, result, err := checkPlanExecuteImpl(ctx, args)
	if err != nil {
		ctx.ToolCall(toolCall).Failure(err.Error())
		return checkPlanToolResult{}, err
	}

	toolCall.Args += fmt.Sprintf(" (%d/%d)", len(plan.Completed()), len(plan.Items))
	ctx.ToolCall(toolCall).Success()

	ctx.UI().PrintPlan()
	return result, nil
}

func checkPlanToolDescribe(args checkPlanToolArgs) iface.ToolCall {
	var indices []string
	for _, index := range args.Indices {
		indices = append(indices, fmt.Sprintf("%v", index))
	}

	return iface.ToolCall{
		Type: "PLAN:CHECK",
		Args: strings.Join(indices, ","),
	}
}

func checkPlanExecuteImpl(ctx iface.Context, args checkPlanToolArgs) (*iface.Plan, checkPlanToolResult, error) {
	plan := ctx.CheckPlanItem(args.Indices...)

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

	return plan, result, nil
}
