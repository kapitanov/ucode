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
	"github.com/kapitanov/ucode/internal/llm"
	"github.com/kapitanov/ucode/internal/tools"
)

var (
	providerType, providerURL, providerAPIKey string
	modelName                                 string
	maxMessages, compacted                    int
)

func init() {
	flag.StringVar(&providerType, "provider", "", "llm provider type (defaults to $LLM_PROVIDER_TYPE)")
	flag.StringVar(&providerURL, "url", "", "llm provider URL (defaults to $LLM_PROVIDER_URL)")
	flag.StringVar(&providerAPIKey, "key", "", "llm provider api key (defaults to $LLM_PROVIDER_API_KEY)")

	flag.StringVar(&modelName, "model", "", fmt.Sprintf("model name (defaults to $OPENROUTER_MODEL or %s)", agent.DefaultModelName))
	flag.IntVar(&maxMessages, "max-messages", agent.DefaultMaxMessages, "max messages before compaction")
	flag.IntVar(&compacted, "compacted-messages", agent.DefaultCompactedMessages, "messages to keep after compaction")

	_ = godotenv.Load()
}

func main() {
	flag.Parse()

	if modelName == "" {
		modelName = os.Getenv("OPENROUTER_MODEL")
	}
	if modelName == "" {
		modelName = agent.DefaultModelName
	}
	if modelName == "" {
		log.Fatal("missing model name (set --model-name or $OPENROUTER_MODEL)")
	}

	if err := tools.CheckDependencies(); err != nil {
		log.Fatal(err)
	}

	llmClient, err := createLLM()
	if err != nil {
		log.Fatal(err)
	}

	a := agent.New(agent.Parameters{
		LLM:               llmClient,
		ModelName:         modelName,
		MaxMessages:       maxMessages,
		CompactedMessages: compacted,
	})
	runAgent(a)
}

func createLLM() (agent.LLMClient, error) {
	switch providerType {
	case "openrouter":
		if providerURL == "" {
			providerURL = os.Getenv("OPENROUTER_API_URL")
		}
		if providerAPIKey == "" {
			providerAPIKey = os.Getenv("OPENROUTER_API_KEY")
		}
		if providerAPIKey == "" {
			return nil, fmt.Errorf("missing openrouter api key (set --key or $OPENROUTER_API_KEY)")
		}

		return llm.NewOpenRouterClient(providerURL, providerAPIKey), nil

	case "ollama":
		if providerURL == "" {
			providerURL = os.Getenv("OLLAMA_URL")
		}
		return llm.NewOllamaClient(providerURL), nil

	case "openai":
		if providerURL == "" {
			providerURL = os.Getenv("OPENAI_API_URL")
		}
		if providerAPIKey == "" {
			providerAPIKey = os.Getenv("OPENAI_API_KEY")
		}
		if providerAPIKey == "" {
			return nil, fmt.Errorf("missing openai api key (set --key or $OPENAI_API_KEY)")
		}
		return llm.NewOpenAIClient(providerURL, providerAPIKey), nil

	default:
		return nil, fmt.Errorf("%q is not a valid provider", providerType)
	}
}

func runAgent(a *agent.Agent) {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		printCursor()

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				log.Printf("Error reading input: %v", err)
			}
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
