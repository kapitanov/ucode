package definitions

import (
	_ "embed"

	"github.com/kapitanov/ucode/internal/tools"
)

var (
	//go:embed prompts/_shared.md
	sharedPrompt string
)

type Agent struct {
	Name   string
	Role   string
	Prompt string
	Tools  []tools.Name
}
