package tui

import (
	"fmt"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/fatih/color"
	"github.com/kapitanov/ucode/internal/tools"
)

var (
	infoStyle       = color.New(color.FgCyan, color.Faint).SprintFunc()
	reasoningStyle  = color.New(color.FgHiCyan).SprintFunc()
	toolStyle       = color.New(color.FgHiYellow, color.Faint).SprintFunc()
	errorStyle      = color.New(color.FgHiRed).SprintFunc()
	compactionStyle = color.New(color.FgHiMagenta, color.Faint).SprintFunc()
	usageStyle      = color.New(color.FgWhite, color.Faint).SprintFunc()
)

func Info(msg string) {
	_, _ = fmt.Fprintf(color.Output, "%s\n", infoStyle(fmt.Sprintf("# %s", msg)))
}

func Message(msg string) {
	markdown, err := renderMarkdown(msg)
	if err != nil {
		markdown = fmt.Sprintf("%s %s\n", cursorStyle("<"), msg)
	}

	_, _ = fmt.Fprint(color.Output, markdown)
}

func renderMarkdown(input string) (string, error) {
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath(styles.DraculaStyle),
		glamour.WithWordWrap(150),
	)
	if err != nil {
		return "", err
	}

	output, err := r.Render(input)
	if err != nil {
		return "", err
	}

	return output, nil
}

func Reasoning(msg string) {
	_, _ = fmt.Fprintf(color.Output, "%s\n", reasoningStyle(msg))
}

func ToolCall(name, args string) {
	toolName := tools.Describe(name, args)
	_, _ = fmt.Fprintf(color.Output, "%s\n", toolStyle(toolName))
}

func ToolCallError(msg string) {
	_, _ = fmt.Fprintf(color.Output, "%s\n", errorStyle(msg))
}

func Compaction(before, after int) {
	_, _ = fmt.Fprintf(color.Output, "%s\n", compactionStyle(fmt.Sprintf("[compaction: %d -> %d messages]", before, after)))
}

func Error(msg string) {
	_, _ = fmt.Fprintf(color.Output, "%s\n", errorStyle(msg))
}

func Usage(tokens int, cost float64) {
	str := fmt.Sprintf("[usage: %d tokens, $%.4f]", tokens, cost)

	_, _ = fmt.Fprintf(color.Output, "%s\n", usageStyle(str))
}
