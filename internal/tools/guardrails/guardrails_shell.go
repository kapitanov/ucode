package guardrails

import (
	"fmt"
	"strings"

	"github.com/kapitanov/ucode/internal/iface"
)

func IsAllowedCommand(ctx iface.Context, command string) bool {
	lines := strings.Split(command, "\n")
	for i := range lines {
		lines[i] = fmt.Sprintf("| %s", lines[i])
	}
	question := fmt.Sprintf("Do you want to allow the following command to be executed?\n%s", strings.Join(lines, "\n"))

	return ctx.Ask(question, []string{"Allow", "Forbid"}) == 0
}
