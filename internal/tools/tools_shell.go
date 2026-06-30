package tools

import (
	"errors"
	"fmt"
	"os/exec"
)

func init() {
	defineTool("shell", "execute a shell command", shellTool, shellToolDescribe)
}

type (
	shellToolArgs struct {
		Cmd string `json:"cmd" description:"command to execute"`
	}

	shellToolResult struct {
		Output   string `json:"output" description:"output of the shell command"`
		ExitCode int    `json:"exit_code" description:"exit code of the shell command"`
	}
)

func shellTool(args shellToolArgs) (shellToolResult, error) {
	cmd := exec.Command("sh", "-c", args.Cmd)
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

func shellToolDescribe(args shellToolArgs) string {
	return fmt.Sprintf("SHELL %s", args.Cmd)
}
