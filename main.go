package main

import (
	"flag"
	"os"

	"github.com/kapitanov/ucode/internal/agents"
	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/llm"
	"github.com/kapitanov/ucode/internal/runner"
	"github.com/kapitanov/ucode/internal/sandbox"
	"github.com/kapitanov/ucode/internal/tui"
)

var (
	providerURL, providerAPIKey, providerModel string
	enableSandbox, printReasoning              bool
)

func init() {
	flag.StringVar(&providerURL, "url", "", "llm provider URL (defaults to $OPENROUTER_URL)")
	flag.StringVar(&providerAPIKey, "key", "", "llm provider api key (defaults to $OPENROUTER_API_KEY)")
	flag.StringVar(&providerModel, "model", "", "llm provider model (defaults to $OPENROUTER_MODEL)")
	flag.BoolVar(&enableSandbox, "sandbox", false, "enable sandbox mode")
	flag.BoolVar(&printReasoning, "reasoning", false, "enable reasoning output")
}

func main() {
	flag.Parse()

	configureWD()
	llmConn, defaultModel := configureLLM()

	agentsUI := tui.New(llmConn, printReasoning)
	defer agentsUI.Close()

	agentsSandbox := configureSandbox(agentsUI)
	agentsRegistry := agents.New(llmConn, defaultModel)

	err := runner.RunAgent(agentsSandbox, agentsRegistry, agentsUI, agentsRegistry.Default())
	if err != nil {
		panic(err)
	}
}

func configureWD() {
	if flag.NArg() > 2 {
		flag.Usage()
		os.Exit(1)
	}

	if flag.NArg() == 1 {
		err := os.Chdir(flag.Arg(0))
		if err != nil {
			panic(err)
		}
	}
}

func configureLLM() (iface.LLM, string) {
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

func configureSandbox(agentsUI iface.UI) iface.Sandbox {
	if enableSandbox {
		return sandbox.Isolated()
	}

	return sandbox.Direct(agentsUI)
}
