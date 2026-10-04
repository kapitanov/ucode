package tools

import (
	"fmt"
	"strings"

	"github.com/kapitanov/ucode/internal/iface"
)

const Shell Name = "shell"

func init() {
	register(Shell, "execute a shell command", shellToolExecute)
}

type (
	shellToolArgs struct {
		Command []string `json:"command" jsonschema_description:"shell command to execute, split into parts"`
	}

	shellToolResult struct {
		Output   string `json:"output"    jsonschema_description:"output of the shell command"`
		ExitCode int    `json:"exit_code" jsonschema_description:"exit code of the shell command"`
	}
)

func shellToolExecute(ctx iface.Context, args shellToolArgs) (shellToolResult, error) {
	toolCall := shellToolDescribe(args)

	if len(args.Command) < 1 {
		ctx.ToolCall(toolCall).Failure("shell command is malformed")
		return shellToolResult{}, fmt.Errorf("shell command is malformed")
	}

	if !shellToolIsAllowedCommand(ctx, args.Command[0]) {
		ctx.ToolCall(toolCall).Failure("shell command execution not allowed")
		return shellToolResult{}, fmt.Errorf("shell command execution not allowed")
	}

	callToken := ctx.ToolCall(toolCall)
	result, err := shellToolExecuteImpl(ctx, args)
	if err != nil {
		callToken.Failure(err.Error())
		return shellToolResult{}, err
	}
	callToken.Success()

	return result, nil
}

func shellToolIsAllowedCommand(ctx iface.Context, command string) bool {
	if !ctx.Sandbox().RequireManualValidation() {
		return true
	}

	lines := strings.Split(command, "\n")
	for i := range lines {
		lines[i] = fmt.Sprintf("| %s", lines[i])
	}
	question := fmt.Sprintf("Do you want to allow the following command to be executed?\n%s", strings.Join(lines, "\n"))

	return ctx.Ask(question, []string{"Allow", "Forbid"}) == 0
}

func shellToolDescribe(args shellToolArgs) iface.ToolCall {
	return iface.ToolCall{
		Type: "SHELL",
		Args: strings.Join(args.Command, " "),
	}
}

func shellToolExecuteImpl(ctx iface.Context, args shellToolArgs) (shellToolResult, error) {
	output, exitCode, err := ctx.Sandbox().ShellCommand(args.Command)
	if err != nil {
		return shellToolResult{}, err
	}

	return shellToolResult{
		Output:   output,
		ExitCode: exitCode,
	}, nil
}
