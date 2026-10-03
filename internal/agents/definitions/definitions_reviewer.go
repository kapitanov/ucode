package definitions

import (
	_ "embed"

	"github.com/kapitanov/ucode/internal/tools"
)

var (
	//go:embed prompts/reviewer.md
	reviewerPrompt string

	Reviewer = Agent{
		Name:   "Семеныч",
		Role:   "reviewer",
		Prompt: sharedPrompt + "\n\n\n" + reviewerPrompt,
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
