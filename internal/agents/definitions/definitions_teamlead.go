package definitions

import (
	_ "embed"

	"github.com/kapitanov/ucode/internal/tools"
)

var (
	//go:embed prompts/teamlead.md
	teamLeadPrompt string

	TeamLead = Agent{
		Name:   "Михалыч",
		Role:   "teamlead",
		Prompt: sharedPrompt + "\n\n\n" + teamLeadPrompt,
		Tools: []tools.Name{
			// User communication
			tools.AskUser,

			// Planning
			tools.CheckPlan,
			tools.ClearPlan,
			tools.ReadPlan,
			tools.WritePlan,

			// Memory
			tools.ListMemory,
			tools.ReadMemory,
			tools.WriteMemory,

			// Subagents
			tools.RunSubagent,

			// Reading files
			tools.GrepFiles,
			tools.ListFiles,
			tools.ReadFile,
		},
	}
)
