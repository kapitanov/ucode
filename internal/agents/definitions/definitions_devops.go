package definitions

import (
	_ "embed"

	"github.com/kapitanov/ucode/internal/tools"
)

var (
	//go:embed prompts/devops.md
	devOpsPrompt string

	DevOps = Agent{
		Name:   "Саныч",
		Role:   "devops",
		Prompt: sharedPrompt + "\n\n\n" + devOpsPrompt,
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
