package definitions

import (
	_ "embed"

	"github.com/kapitanov/ucode/internal/tools"
)

var (
	//go:embed prompts/tester.md
	testerPrompt string

	Tester = Agent{
		Name:   "Аркадич",
		Role:   "tester",
		Prompt: sharedPrompt + "\n\n\n" + testerPrompt,
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
