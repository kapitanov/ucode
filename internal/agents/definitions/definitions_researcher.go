package definitions

import (
	_ "embed"

	"github.com/kapitanov/ucode/internal/tools"
)

var (
	//go:embed prompts/researcher.md
	researcherPrompt string

	Researcher = Agent{
		Name:   "Васильич",
		Role:   "researcher",
		Prompt: sharedPrompt + "\n\n\n" + researcherPrompt,
		Tools: []tools.Name{
			// User communication
			tools.AskUser,

			// Memory
			tools.ListMemory,
			tools.ReadMemory,
			tools.WriteMemory,

			// Reading files
			tools.GrepFiles,
			tools.ListFiles,
			tools.ReadFile,

			// Writing files
			tools.EditFile,
			tools.RemoveFile,
			tools.WriteFile,
		},
	}
)
