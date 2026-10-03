package definitions

import (
	_ "embed"

	"github.com/kapitanov/ucode/internal/tools"
)

var (
	//go:embed prompts/architect.md
	architectPrompt string

	Architect = Agent{
		Name:   "Кульман",
		Role:   "architect",
		Prompt: sharedPrompt + "\n\n\n" + architectPrompt,
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

			// Shell
			tools.Shell,
		},
	}
)
