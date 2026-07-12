package tools

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tools/guardrails"
)

func init() {
	register("shell", "execute a shell command", shellToolExecute)
}

type (
	shellToolArgs struct {
		Command string `json:"command" jsonschema_description:"shell command to execute"`
	}

	shellToolResult struct {
		Output   string `json:"output"    jsonschema_description:"output of the shell command"`
		ExitCode int    `json:"exit_code" jsonschema_description:"exit code of the shell command"`
	}
)

func shellToolExecute(ctx iface.Context, args shellToolArgs) (shellToolResult, error) {
	toolCall := shellToolDescribe(args)

	if !guardrails.IsAllowedCommand(ctx, args.Command) {
		ctx.ToolCall(toolCall).Failure("shell command execution not allowed")
		return shellToolResult{}, fmt.Errorf("shell command execution not allowed")
	}

	callToken := ctx.ToolCall(toolCall)
	result, err := shellToolExecuteImpl(args)
	if err != nil {
		callToken.Failure(err.Error())
		return shellToolResult{}, err
	}
	callToken.Success()

	return result, nil
}

func shellToolDescribe(args shellToolArgs) iface.ToolCall {
	return iface.ToolCall{
		Type: "SHELL",
		Args: args.Command,
	}
}

func shellToolExecuteImpl(args shellToolArgs) (shellToolResult, error) {
	cmd := exec.Command("sh", "-c", args.Command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return shellToolResult{
				Output:   string(output),
				ExitCode: exitErr.ExitCode(),
			}, nil
		}

		return shellToolResult{}, fmt.Errorf("command failed with error: %s\nOutput: %s", err.Error(), string(output))
	}

	return shellToolResult{
		Output:   string(output),
		ExitCode: 0,
	}, nil
}
