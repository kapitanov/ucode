package tools

import (
	"fmt"
	"strings"

	"github.com/kapitanov/ucode/internal/iface"
)

const AskUser Name = "ask_user"

func init() {
	register(AskUser, "ask user a question and let him select one of predefined options", askUserToolExecute)
}

type (
	askUserToolArgs struct {
		Question string   `json:"question" jsonschema_description:"question to ask the user"`
		Options  []string `json:"options"  jsonschema_description:"predefined options for the user to choose from"`
	}

	askUserToolResult struct {
		Answer string `json:"answer" jsonschema_description:"option selected by the user"`
	}
)

func askUserToolExecute(ctx iface.Context, args askUserToolArgs) (askUserToolResult, error) {
	index := ctx.Ask(args.Question, args.Options)
	answer := args.Options[index]

	toolCall := askUserToolDescribe(args)
	ctx.ToolCall(toolCall).Success()

	return askUserToolResult{Answer: answer}, nil
}

func askUserToolDescribe(args askUserToolArgs) iface.ToolCall {
	var options []string
	for _, option := range args.Options {
		options = append(options, fmt.Sprintf("%q", option))
	}

	return iface.ToolCall{
		Type: "ASK",
		Args: fmt.Sprintf("%s? (%s)", args.Question, strings.Join(options, "/")),
	}
}
