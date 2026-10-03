package tools

import (
	"github.com/kapitanov/ucode/internal/iface"
)

const ReadPlan Name = "read_plan"

func init() {
	register(ReadPlan, "read agent's plan", readPlanToolExecute)
}

type (
	readPlanToolArgs struct{}

	readPlanToolResult struct {
		Items []readPlanToolResultItem `json:"items" jsonschema_description:"plan items"`
	}

	readPlanToolResultItem struct {
		Index int    `json:"index" jsonschema_description:"plan item index"`
		Done  bool   `json:"done"  jsonschema_description:"plan item completion status"`
		Title string `json:"title" jsonschema_description:"plan item title"`
	}
)

func readPlanToolExecute(ctx iface.Context, args readPlanToolArgs) (readPlanToolResult, error) {
	toolCall := readPlanToolDescribe(args)

	callToken := ctx.ToolCall(toolCall)
	result, err := readPlanExecuteImpl(ctx, args)
	if err != nil {
		callToken.Failure(err.Error())
		return readPlanToolResult{}, err
	}
	callToken.Success()

	return result, nil
}

func readPlanToolDescribe(_ readPlanToolArgs) iface.ToolCall {
	return iface.ToolCall{
		Type: "PLAN:READ",
		Args: "",
	}
}

func readPlanExecuteImpl(ctx iface.Context, _ readPlanToolArgs) (readPlanToolResult, error) {
	plan := ctx.Plan()

	result := readPlanToolResult{
		Items: make([]readPlanToolResultItem, len(plan.Items)),
	}
	for i := range plan.Items {
		result.Items[i] = readPlanToolResultItem{
			Index: plan.Items[i].Index,
			Done:  plan.Items[i].Done,
			Title: plan.Items[i].Title,
		}
	}

	return result, nil
}
