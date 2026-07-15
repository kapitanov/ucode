package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/kapitanov/ucode/internal/iface"
	"golang.org/x/term"
)

var (
	printStyle    = color.New(color.FgYellow, color.Italic).SprintFunc()
	cursorStyle   = color.New(color.FgHiGreen, color.Bold).SprintFunc()
	thinkingStyle = color.New(color.FgHiYellow).SprintFunc()

	reasoningAgentNameStyle = color.New(color.FgCyan, color.Bold, color.Italic).SprintFunc()
	reasoningMessageStyle   = color.New(color.FgCyan, color.Italic).SprintFunc()
	responseAgentNameStyle  = color.New(color.FgHiWhite, color.Bold).SprintFunc()
	responseMessageStyle    = color.New(color.FgHiWhite).SprintFunc()
	refusalAgentNameStyle   = color.New(color.FgHiRed, color.Bold).SprintFunc()
	refusalMessageStyle     = color.New(color.FgHiRed).SprintFunc()

	runningToolStyle    = color.New(color.FgYellow, color.Faint).SprintFunc()
	successfulToolStyle = color.New(color.FgGreen, color.Faint).SprintFunc()
	failureToolStyle    = color.New(color.FgHiRed, color.Faint).SprintFunc()

	hintStyle         = color.New(color.FgWhite, color.Faint).SprintFunc()
	outputHeaderStyle = color.New(color.FgHiWhite, color.Bold).SprintFunc()
	outputItemStyle   = color.New(color.FgHiWhite).SprintFunc()

	questionStyle         = color.New(color.FgHiMagenta).SprintFunc()
	selectedQuestionStyle = color.New(color.FgHiMagenta, color.Underline).SprintFunc()
	errorStyle            = color.New(color.FgRed).SprintFunc()
)

func Printf(format string, a ...any) {
	_, _ = fmt.Fprintf(color.Output, "%s\n", printStyle(fmt.Sprintf(format, a...)))
}

type UI struct {
	plan           iface.Plan
	memory         *iface.Memory
	llm            iface.LLM
	printReasoning bool
}

func New(llm iface.LLM, printReasoning bool) *UI {
	return &UI{
		memory:         &iface.Memory{},
		llm:            llm,
		printReasoning: printReasoning,
	}
}

func (u *UI) Prompt() string {
	for {
		str := u.prompt()

		var exit bool
		if u.tryHandleCommand(str, &exit) {
			if exit {
				return ""
			}
			continue
		}

		return str
	}
}

func (u *UI) prompt() string {
	rl, err := readline.New(fmt.Sprintf("%s ", cursorStyle(">")))
	if err != nil {
		panic(err)
	}
	defer func() { _ = rl.Close() }()

	for {
		str, err := rl.Readline()
		if err != nil { // io.EOF
			return ""
		}

		str = strings.TrimSpace(str)

		if str == "" {
			continue
		}

		return str
	}
}

func (u *UI) Ask(question string, options []string) int {
	_, _ = fmt.Fprintf(color.Output, "%s\n", questionStyle(question))

	selectedIndex := 0
	renderLine := func() {
		_, _ = fmt.Fprint(os.Stdout, "\r\x1b[2K") //ansi clear line
		for i, option := range options {
			before, after, style := " ", " ", questionStyle
			if i == selectedIndex {
				before, after, style = "<", ">", selectedQuestionStyle
			}
			_, _ = fmt.Fprintf(color.Output, "%s ", style(fmt.Sprintf("%s%s%s", before, option, after)))
		}
	}
	renderLine()

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		panic(err)
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	keyBuf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(keyBuf)
		if err != nil {
			panic(err)
		}

		if n == 1 && keyBuf[0] == 13 { // Enter key
			linesToClear := len(strings.Split(question, "\n"))
			for i := 0; i < linesToClear; i++ {
				_, _ = fmt.Fprint(os.Stdout, "\r\x1b[2K\x1b[1A") //ansi clear line and move cursor up
			}
			return selectedIndex
		}

		if n == 3 && keyBuf[0] == 27 && keyBuf[1] == 91 { // Arrow keys
			switch keyBuf[2] {
			case 68: // Left arrow
				if selectedIndex > 0 {
					selectedIndex--
					renderLine()
				}

			case 67: // Right arrow
				if selectedIndex < len(options)-1 {
					selectedIndex++
					renderLine()
				}
			}
		}
	}
}

func (u *UI) Thinking() iface.ThinkingToken {
	s := spinner.New(
		spinner.CharSets[57],
		100*time.Millisecond,
		spinner.WithWriter(color.Output),
		spinner.WithSuffix(thinkingStyle(" Thinking...")),
	)
	s.Start()

	return &thinkingToken{
		spinner: s,
	}
}

type thinkingToken struct {
	spinner *spinner.Spinner
}

func (t *thinkingToken) Done() {
	t.spinner.Stop()
}

func (u *UI) Reasoning(agentName, response string) {
	if !u.printReasoning {
		return
	}

	u.printAgentResponse(agentName, response, reasoningAgentNameStyle, reasoningMessageStyle)
}

func (u *UI) Response(agentName, response string) {
	u.printAgentResponse(agentName, response, responseAgentNameStyle, responseMessageStyle)
}

func (u *UI) Refusal(agentName, response string) {
	u.printAgentResponse(agentName, response, refusalAgentNameStyle, refusalMessageStyle)
}

func (u *UI) printAgentResponse(agentName, response string, nameStyle, textStyle func(a ...any) string) {
	for _, line := range strings.Split(response, "\n") {
		if agentName == "" {
			_, _ = fmt.Fprintf(color.Output, "%s\n", textStyle(line))
		} else {
			_, _ = fmt.Fprintf(color.Output, "%s: %s\n", nameStyle(agentName), textStyle(line))
		}
	}
}

func (u *UI) ToolCall(toolCall iface.ToolCall) iface.ToolCallToken {
	shortArgs := strings.Split(toolCall.Args, "\n")[0]
	s := spinner.New(
		spinner.CharSets[57],
		100*time.Millisecond,
		spinner.WithWriter(color.Output),
		spinner.WithSuffix(runningToolStyle(fmt.Sprintf(" %s %s", toolCall.Type, shortArgs))),
	)
	s.Start()

	return &toolCallToken{
		toolCall: toolCall,
		spinner:  s,
	}
}

type toolCallToken struct {
	toolCall iface.ToolCall
	spinner  *spinner.Spinner
}

func (t *toolCallToken) Success() {
	t.spinner.Stop()

	_, _ = fmt.Fprintf(color.Output, "%s\n", successfulToolStyle(fmt.Sprintf("%s %s", t.toolCall.Type, t.toolCall.Args)))
}

func (t *toolCallToken) Failure(reason string) {
	t.spinner.Stop()

	_, _ = fmt.Fprintf(color.Output, "%s\n", failureToolStyle(fmt.Sprintf("%s %s", t.toolCall.Type, t.toolCall.Args)))
	_, _ = fmt.Fprintf(color.Output, "%s\n", failureToolStyle(reason))
}

func (u *UI) SetPlan(plan iface.Plan) {
	u.plan = plan
	u.printHint(fmt.Sprintf("Plan has been updated (%d/%d)", len(plan.Completed()), len(plan.Items)))
	u.printPlan()
}

func (u *UI) SetMemory(memory *iface.Memory) {
	u.memory = memory
	u.printHint(fmt.Sprintf("Memory has been updated (%d)", len(memory.Items)))
}

func (u *UI) printHint(str string) {
	_, _ = fmt.Fprintf(color.Output, "%s\n", hintStyle(str))
}

func (u *UI) Close() {}
