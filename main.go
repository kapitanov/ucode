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

const DefaultModel = "openrouter/free"

// const DefaultModel = "anthropic/claude-haiku-4.5"
// const DefaultModel = "tencent/hy3:free"
// const DefaultModel = "qwen/qwen3-next-80b-a3b-instruct:free"

func main() {
	cfg := configure()

	configureWorkDir()
	llmConn := configureLLM(cfg.ProviderURL, cfg.ProviderAPIKey, cfg.ProviderModel)

	agentsUI := configureUI(llmConn, cfg.PrintReasoning, cfg.UseModernUI)
	defer agentsUI.Close()

	agentsSandbox := configureSandbox(agentsUI, cfg.EnableSandbox)
	agentsRegistry := agents.New(llmConn, cfg.ProviderModel)

	err := runner.RunAgent(agentsSandbox, agentsRegistry, agentsUI, agentsRegistry.Default())
	if err != nil {
		panic(err)
	}
}

func configureWorkDir() {
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

func configureLLM(providerURL, providerAPIKey, providerModel string) iface.LLM {
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

	return llm.NewOpenRouterClient(providerURL, providerAPIKey)
}

func configureUI(llm iface.LLM, printReasoning, useModernUI bool) iface.UI {
	// TODO here we can add a flag to choose between plain TUI and pretty TUI, for now we will use plain TUI
	_ = useModernUI

	return tui.New(llm, printReasoning)
}

func configureSandbox(agentsUI iface.UI, enableSandbox bool) iface.Sandbox {
	if enableSandbox {
		return sandbox.Isolated()
	}

	return sandbox.Direct(agentsUI)
}

type configuration struct {
	ProviderURL, ProviderAPIKey, ProviderModel string
	EnableSandbox, PrintReasoning, UseModernUI bool
}

func configure() configuration {
	var cfg configuration

	flag.StringVar(&cfg.ProviderURL, "url", "", "llm provider URL (defaults to $OPENROUTER_URL)")
	flag.StringVar(&cfg.ProviderAPIKey, "key", "", "llm provider api key (defaults to $OPENROUTER_API_KEY)")
	flag.StringVar(&cfg.ProviderModel, "model", "", "llm provider model (defaults to $OPENROUTER_MODEL)")
	flag.BoolVar(&cfg.EnableSandbox, "sandbox", false, "enable sandbox mode")
	flag.BoolVar(&cfg.PrintReasoning, "reasoning", false, "enable reasoning output")
	flag.BoolVar(&cfg.UseModernUI, "modern-ui", false, "use modern UI (not implemented yet)")

	flag.Parse()

	if cfg.ProviderURL == "" {
		cfg.ProviderURL = os.Getenv("OPENROUTER_URL")
	}
	if cfg.ProviderAPIKey == "" {
		cfg.ProviderAPIKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if cfg.ProviderModel == "" {
		cfg.ProviderModel = os.Getenv("OPENROUTER_MODEL")

		if cfg.ProviderModel == "" {
			cfg.ProviderModel = DefaultModel
		}
	}

	if cfg.ProviderURL == "" && cfg.ProviderAPIKey == "" {
		panic("LLM provider URL or/and API key must be provided via flags or environment variables")
	}

	return cfg
}
