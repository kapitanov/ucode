package definitions

import (
	_ "embed"

	"github.com/kapitanov/ucode/internal/tools"
)

var (
	//go:embed prompts/developer.md
	developerPrompt string

	Developer = Agent{
		Name:   "Петрович",
		Role:   "developer",
		Prompt: sharedPrompt + "\n\n\n" + developerPrompt,
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

			// Shell
			tools.Shell,
		},
	}
)
