package definitions

import (
	_ "embed"

	"github.com/kapitanov/ucode/internal/tools"
)

var (
	//go:embed prompts/documentation.md
	documentationPrompt string

	Documentation = Agent{
		Name:   "Иваныч",
		Role:   "documentation",
		Prompt: sharedPrompt + "\n\n\n" + documentationPrompt,
		Tools: []tools.Name{
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
