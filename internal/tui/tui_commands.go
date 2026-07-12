package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/fatih/color"
)

func (u *UI) tryHandleCommand(str string, exit *bool) bool {
	str = strings.TrimSpace(str)
	if str == "" || !strings.HasPrefix(str, "/") {
		return false
	}

	switch strings.TrimSpace(str) {
	case "/exit", "/quit":
		*exit = true
		return true

	case "/plan":
		u.printPlan()
		return true

	case "/memory":
		u.printMemory()
		return true

	case "/usage":
		u.printUsage()
		return true

	default:
		_, _ = fmt.Fprintf(color.Output, "%s\n", errorStyle(fmt.Sprintf("Invalid command: %q", str)))
		return true
	}
}

func (u *UI) printPlan() {
	_, _ = fmt.Fprintf(color.Output, "%s (%d/%d)\n", outputHeaderStyle("PLAN"), len(u.plan.Completed()), len(u.plan.Items))

	width := len(fmt.Sprintf("%d", len(u.plan.Items)+1))

	for _, item := range u.plan.Items {
		status := " "
		if item.Done {
			status = "x"
		}

		_, _ = fmt.Fprintf(color.Output, "  %s) [%s] %s\n", outputItemStyle(fmt.Sprintf("%*d", width, item.Index)), status, outputItemStyle(item.Title))
	}
}

func (u *UI) printMemory() {
	_, _ = fmt.Fprintf(color.Output, "%s (%d)\n", outputHeaderStyle("MEMORY"), len(u.memory.Items))
	for _, item := range u.memory.Items {
		_, _ = fmt.Fprintf(color.Output, "  %s: %s\n", outputItemStyle(item.Key), outputItemStyle(item.Value))
	}
}

func (u *UI) printUsage() {
	usage := u.llm.Usage()

	_, _ = fmt.Fprintf(
		color.Output,
		"%s %s, %s\n",
		outputHeaderStyle("USAGE:"),
		outputItemStyle(tokens(int64(usage.TotalTokens))),
		outputItemStyle(fmt.Sprintf("$%0.2f", usage.Cost)),
	)
}

func tokens(s int64) string {
	symbols := []string{" tokens", "K tokens", "M tokens"}

	i := math.Floor(
		math.Log(float64(s)) / math.Log(1024),
	)
	if s < 10 {
		return fmt.Sprintf("%d tokens", s)
	}
	size := float64(s) / math.Pow(1024, math.Floor(i))
	format := "%.0f"
	if size < 10 {
		format = "%.1f"
	}

	return fmt.Sprintf(format+"%s", size, symbols[int(i)])
}
