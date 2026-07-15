package iface

import (
	"context"

	"github.com/revrost/go-openrouter"
)

type LLM interface {
	Usage() openrouter.Usage
	CreateChatCompletion(ctx context.Context, req openrouter.ChatCompletionRequest) (*openrouter.ChatCompletionResponse, error)
}

type Agents interface {
	ByRole(role string) Agent
	Default() Agent
}

type Agent interface {
	Name() string
	Run(ctx Context) error
}

type Context interface {
	// Input
	Prompt() string
	Ask(question string, options []string) int
	Thinking() ThinkingToken

	// Output
	Response(response string)
	Reasoning(response string)
	Refusal(response string)
	ToolCall(toolCall ToolCall) ToolCallToken

	// Planning
	Plan() Plan
	ClearPlan()
	WritePlan(items []string) Plan
	CheckPlanItem(index int) Plan

	// Memory
	Memory() Memory
	WriteMemory(key, value string) Memory

	// Subagents
	Agents() Agents
	Sandbox() Sandbox
	RunSubagent(agent Agent, request string) (string, error)
}

type UI interface {
	// Input
	Prompt() string
	Ask(question string, options []string) int
	Thinking() ThinkingToken

	// Output
	Response(agentName, response string)
	Reasoning(agentName, response string)
	Refusal(agentName, response string)
	ToolCall(toolCall ToolCall) ToolCallToken

	SetPlan(plan Plan)
	SetMemory(memory *Memory)
}

type ThinkingToken interface {
	Done()
}

type ToolCall struct {
	Type string
	Args string
}

type ToolCallToken interface {
	Success()
	Failure(message string)
}

type Plan struct {
	Items []PlanItem
}

func (p Plan) Completed() []PlanItem {
	var completed []PlanItem
	for _, item := range p.Items {
		if item.Done {
			completed = append(completed, item)
		}
	}

	return completed
}

type PlanItem struct {
	Index int
	Done  bool
	Title string
}

type Memory struct {
	Items []MemoryItem
}

type MemoryItem struct {
	Key   string
	Value string
}

type Sandbox interface {
	RequireManualValidation() bool
	ReadFile(path string) (bs []byte, err error)
	ListFiles(dir string) (dirs []string, files []string, err error)
	SearchFiles(pattern string, path, fileType *string, caseSensitive *bool) (results []string, err error)
	WriteFile(path string, bs []byte) (err error)
	PatchFile(path, diff string) (bs []byte, err error)
	RemoveFile(path string) (err error)
	ShellCommand(command string) (output string, exitCode int, err error)
}
