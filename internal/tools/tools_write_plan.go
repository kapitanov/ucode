package tools

import (
	"fmt"
	"strings"

	"github.com/kapitanov/ucode/internal/iface"
)

const WritePlan Name = "write_plan"

func init() {
	register(WritePlan, "write agent's plan (existing plan items will be preserved)", writePlanToolExecute)
}

type (
	writePlanToolArgs struct {
		Items []writePlanToolArgsItem `json:"items" jsonschema_description:"plan items"`
	}

	writePlanToolArgsItem struct {
		Title string `json:"title" jsonschema_description:"plan item title"`
	}

	writePlanToolResult struct {
		Items []writePlanToolResultItem `json:"items" jsonschema_description:"plan items"`
	}

	writePlanToolResultItem struct {
		Index int    `json:"index" jsonschema_description:"plan item index"`
		Done  bool   `json:"done"  jsonschema_description:"plan item completion status"`
		Title string `json:"title" jsonschema_description:"plan item title"`
	}
)

func writePlanToolExecute(ctx iface.Context, args writePlanToolArgs) (writePlanToolResult, error) {
	result, err := writePlanExecuteImpl(ctx, args)
	if err != nil {
		toolCall := writePlanToolDescribe(args)
		callToken := ctx.ToolCall(toolCall)
		callToken.Failure(err.Error())
		return writePlanToolResult{}, err
	}

	ctx.UI().PrintPlan()
	return result, nil
}

func writePlanToolDescribe(args writePlanToolArgs) iface.ToolCall {
	var items []string
	for _, item := range args.Items {
		items = append(items, fmt.Sprintf("%q", item.Title))
	}

	return iface.ToolCall{
		Type: "PLAN:WRITE",
		Args: strings.Join(items, ", "),
	}
}

func writePlanExecuteImpl(ctx iface.Context, args writePlanToolArgs) (writePlanToolResult, error) {
	var items []string
	for _, item := range args.Items {
		items = append(items, item.Title)
	}
	ctx.WritePlan(items)

	plan := ctx.Plan()
	result := writePlanToolResult{
		Items: make([]writePlanToolResultItem, len(plan.Items)),
	}
	for i := range plan.Items {
		result.Items[i] = writePlanToolResultItem{
			Index: plan.Items[i].Index,
			Done:  plan.Items[i].Done,
			Title: plan.Items[i].Title,
		}
	}

	return result, nil
}
