package main

import (
	"flag"
	"os"

	"github.com/kapitanov/ucode/internal/agents"
	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/llm"
	"github.com/kapitanov/ucode/internal/runner"
	"github.com/kapitanov/ucode/internal/tui"
)

func main() {
	llmConn, defaultModel := configureLLM()

	agentsUI := tui.New(llmConn)
	defer agentsUI.Close()

	agentsRegistry := agents.New(llmConn, defaultModel)

	runner.RunAgent(agentsRegistry, agentsUI, agentsRegistry.Default())
}

func configureLLM() (iface.LLM, string) {
	var providerURL, providerAPIKey, providerModel string
	flag.StringVar(&providerURL, "url", "", "llm provider URL (defaults to $OPENROUTER_URL)")
	flag.StringVar(&providerAPIKey, "key", "", "llm provider api key (defaults to $OPENROUTER_API_KEY)")
	flag.StringVar(&providerModel, "model", "", "llm provider model (defaults to $OPENROUTER_MODEL)")
	flag.Parse()

	if providerURL == "" {
		providerURL = os.Getenv("OPENROUTER_URL")
	}
	if providerAPIKey == "" {
		providerAPIKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if providerModel == "" {
		providerModel = os.Getenv("OPENROUTER_MODEL")
	}

	if providerURL == "" && providerAPIKey == "" {
		panic("LLM provider URL or/and API key must be provided via flags or environment variables")
	}

	return llm.NewOpenRouterClient(providerURL, providerAPIKey), providerModel
}
