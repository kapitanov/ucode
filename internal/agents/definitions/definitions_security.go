package definitions

import (
	_ "embed"

	"github.com/kapitanov/ucode/internal/tools"
)

var (
	//go:embed prompts/security.md
	securityPrompt string

	Security = Agent{
		Name:   "Дед Терентий",
		Role:   "security",
		Prompt: sharedPrompt + "\n\n\n" + securityPrompt,
		Tools: []tools.Name{
			// Memory
			tools.ListMemory,
			tools.ReadMemory,
			tools.WriteMemory,

			// Reading files
			tools.GrepFiles,
			tools.ListFiles,
			tools.ReadFile,
		},
	}
)
