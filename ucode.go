package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/kapitanov/ucode/internal/agent"
	"github.com/kapitanov/ucode/internal/tools"
)

var (
	apiKey, modelName      string
	maxMessages, compacted int
)

func init() {
	flag.StringVar(&apiKey, "api-key", "", "openrouter api key (defaults to $OPENROUTER_API_KEY)")
	flag.StringVar(&modelName, "model-name", "", fmt.Sprintf("openrouter model name (defaults to $OPENROUTER_MODEL or %s)", agent.DefaultModelName))
	flag.IntVar(&maxMessages, "max-messages", agent.DefaultMaxMessages, "max messages before compaction")
	flag.IntVar(&compacted, "compacted-messages", agent.DefaultCompactedMessages, "messages to keep after compaction")

	godotenv.Load()
}

func main() {
	flag.Parse()

	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if modelName == "" {
		modelName = os.Getenv("OPENROUTER_MODEL")
	}
	if modelName == "" {
		modelName = agent.DefaultModelName
	}

	if apiKey == "" {
		log.Fatal("missing openrouter api key")
	}
	if modelName == "" {
		log.Fatal("missing openrouter model name")
	}

	if err := tools.CheckDependencies(); err != nil {
		log.Fatal(err)
	}

	a := agent.New(agent.Parameters{
		APIKey:            apiKey,
		ModelName:         modelName,
		MaxMessages:       maxMessages,
		CompactedMessages: compacted,
	})
	runAgent(a)
}

func runAgent(a *agent.Agent) {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		printCursor()

		if !scanner.Scan() {
			return
		}

		str := scanner.Text()
		str = strings.TrimSpace(str)

		switch str {
		case "":
			continue
		case "exit", "/exit":
			return
		default:
			runAgentLoop(a, str)
		}
	}
}

func runAgentLoop(a *agent.Agent, str string) {
	s := spinner.New(spinner.CharSets[57], 100*time.Millisecond)
	s.Start()
	defer s.Stop()

	ch := a.Run(str)
	for msg := range ch {
		s.Stop()
		printMsg(msg)
		s.Start()
	}
}

var (
	cursorStyle     = color.New(color.FgHiGreen, color.Bold).SprintFunc()
	reasoningStyle  = color.New(color.FgHiCyan).SprintFunc()
	toolStyle       = color.New(color.FgHiYellow, color.Faint).SprintFunc()
	errorStyle      = color.New(color.FgHiRed).SprintFunc()
	compactionStyle = color.New(color.FgHiMagenta, color.Faint).SprintFunc()
)

func printCursor() {
	_, _ = fmt.Fprintf(color.Output, "%s ", cursorStyle(">"))
}

func printMsg(e agent.Event) {
	if e.Message != nil && e.Message.Text != "" {
		outputText, err := glamour.Render(e.Message.Text, styles.DarkStyle)
		if err != nil {
			outputText = fmt.Sprintf("%s %s\n", cursorStyle("<"), e.Message.Text)
		}

		_, _ = fmt.Fprint(color.Output, outputText)
	}

	if e.Reasoning != nil {
		_, _ = fmt.Fprintf(color.Output, "%s\n", reasoningStyle(e.Reasoning.Text))
	}

	if e.ToolCall != nil {
		toolName := tools.Describe(e.ToolCall.Name, e.ToolCall.Args)
		_, _ = fmt.Fprintf(color.Output, "%s\n", toolStyle(toolName))
	}

	if e.ToolResponse != nil {
		if e.ToolResponse.Error != "" {
			_, _ = fmt.Fprintf(color.Output, "  %s\n", errorStyle(e.ToolResponse.Error))
		}
	}

	if e.Compaction != nil {
		_, _ = fmt.Fprintf(color.Output, "%s\n", compactionStyle(fmt.Sprintf("[compaction: %d -> %d messages]", e.Compaction.Before, e.Compaction.After)))
	}

	if e.Error != nil {
		_, _ = fmt.Fprintf(color.Output, "%s\n", errorStyle(e.Error.Error()))
	}
}
