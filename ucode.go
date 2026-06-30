package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/kapitanov/ucode/internal/agent"
	"github.com/kapitanov/ucode/internal/llm"
	"github.com/kapitanov/ucode/internal/tools"
	"github.com/kapitanov/ucode/internal/tui"
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
	tui.Info(fmt.Sprintf("Using model: %s (%s)", a.Model, a.ProviderType))

	reader := tui.NewReader()

	for {
		str, ok := reader.Next()
		if !ok {
			return
		}

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
	tui.WithSpinner(func(spinner tui.Spinner) {
		ch := a.Run(str)
		for msg := range ch {
			spinner.Pause(func() { printMsg(msg) })
		}
	})
}

func printMsg(e agent.Event) {
	if e.Message != nil && e.Message.Text != "" {
		tui.Message(e.Message.Text)
	}

	if e.Reasoning != nil {
		tui.Reasoning(e.Reasoning.Text)
	}

	if e.ToolCall != nil {
		tui.ToolCall(e.ToolCall.Name, e.ToolCall.Args)
	}

	if e.ToolResponse != nil {
		if e.ToolResponse.Error != "" {
			tui.ToolCallError(e.ToolResponse.Error)
		}
	}

	if e.Compaction != nil {
		tui.Compaction(e.Compaction.Before, e.Compaction.After)
	}

	if e.Usage != nil {
		tui.Usage(e.Usage.Tokens, e.Usage.Cost)
	}

	if e.Error != nil {
		tui.Error(e.Error.Error())
	}
}
