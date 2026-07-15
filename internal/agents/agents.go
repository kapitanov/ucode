package agents

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kapitanov/ucode/internal/agents/compaction"
	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tools"
	"github.com/kapitanov/ucode/internal/tui"
	"github.com/revrost/go-openrouter"
	"go.yaml.in/yaml/v3"
)

const (
	DefaultMaxMessages       = 100
	DefaultCompactedMessages = 25
)

func New(llmConn iface.LLM, defaultModel string) iface.Agents {
	var agentsConfig agentsConfigYAML
	err := yaml.Unmarshal(agentsConfigYAMLBytes, &agentsConfig)
	if err != nil {
		panic(err)
	}

	agents := make(map[string]iface.Agent)
	for _, def := range agentsConfig.Roles {
		prompt := def.Prompt + "\n\n" + agentsConfig.SharedPrompt
		model := def.Model
		if model == "" {
			model = defaultModel
			if model == "" {
				model = agentsConfig.DefaultModel
			}
		}

		agent := newAgentImpl(def.Name, def.Role, prompt, model, def.ForbiddenTools, llmConn)
		agents[def.Name] = agent

		var toolsList []string
		for _, t := range agent.tools {
			toolsList = append(toolsList, t.Function.Name)
		}
		tui.Printf("%% Agent %q (%s):", agent.name, def.Role)
		tui.Printf("%%   Model = %s", model)
		tui.Printf("%%   Tools = %s", strings.Join(toolsList, ", "))
	}

	defaultAgent := agents[agentsConfig.DefaultRole]
	if defaultAgent == nil {
		panic(fmt.Sprintf("default agent %q is not defined", agentsConfig.DefaultRole))
	}

	return &agentsImpl{
		defaultAgent: defaultAgent,
		agents:       agents,
	}
}

var (
	//go:embed agents.yaml
	agentsConfigYAMLBytes []byte
)

type agentsConfigYAML struct {
	DefaultRole  string                     `yaml:"default_role"`
	DefaultModel string                     `yaml:"default_model"`
	SharedPrompt string                     `yaml:"shared_prompt"`
	Roles        map[string]agentConfigYAML `yaml:"roles"`
}

type agentConfigYAML struct {
	Name           string   `yaml:"name"`
	Role           string   `yaml:"role"`
	Prompt         string   `yaml:"prompt"`
	Model          string   `yaml:"model"`
	ForbiddenTools []string `yaml:"forbidden_tools"`
}

type agentsImpl struct {
	defaultAgent iface.Agent
	agents       map[string]iface.Agent
}

func (a *agentsImpl) Default() iface.Agent           { return a.defaultAgent }
func (a *agentsImpl) ByRole(role string) iface.Agent { return a.agents[role] }

type agentImpl struct {
	name, role, prompt, model string
	llm                       iface.LLM
	tools                     []openrouter.Tool

	request           openrouter.ChatCompletionRequest
	maxMessages       int
	compactedMessages int
}

func newAgentImpl(name, role, prompt, model string, forbiddenTools []string, llmConn iface.LLM) *agentImpl {
	var ts []openrouter.Tool
	for _, t := range tools.All() {
		if slices.Contains(forbiddenTools, t.Name) {
			continue
		}

		ts = append(ts, t.Definition)
	}

	return &agentImpl{
		name:   name,
		role:   role,
		prompt: prompt,
		model:  model,
		llm:    llmConn,
		request: openrouter.ChatCompletionRequest{
			Model: model,
			Messages: []openrouter.ChatCompletionMessage{
				openrouter.SystemMessage(prompt),
			},
			Tools: ts,
			Reasoning: &openrouter.ChatCompletionReasoning{
				Enabled: new(true),
			},
		},
		tools:       ts,
		maxMessages: DefaultMaxMessages,
	}
}

func (a *agentImpl) Name() string { return a.name }

func (a *agentImpl) Run(ctx iface.Context) error {
	for {
		req := ctx.Prompt()
		if req == "" {
			return nil
		}

		a.request.Messages = append(a.request.Messages, openrouter.UserMessage(req))
		err := a.runIter(ctx)
		if err != nil {
			return err
		}

		compactionResult := compaction.Compact(a.request)
		if compactionResult != nil {
			a.request = compactionResult.Request
			ctx.Reasoning(fmt.Sprintf("# conversation compacted: %d -> %d", compactionResult.Before, compactionResult.After))
		}
	}
}

func (a *agentImpl) runIter(ctx iface.Context) error {
	for {
		done := false
		err := a.runOne(ctx)
		if err != nil {
			if errors.Is(err, errDone) {
				done = true
			} else {
				return err
			}
		}

		if done {
			return nil
		}
	}
}

var errDone = errors.New("done")

func (a *agentImpl) runOne(c iface.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	thinking := c.Thinking()
	response, err := a.llm.CreateChatCompletion(ctx, a.request)
	thinking.Done()
	if err != nil {
		return err
	}
	if len(response.Choices) == 0 {
		return fmt.Errorf("api returned empty choices")
	}

	msg := response.Choices[0].Message
	a.request.Messages = append(a.request.Messages, msg)

	if msg.Reasoning != nil {
		c.Reasoning(*msg.Reasoning)
	}

	if msg.Content.Text != "" {
		c.Response(msg.Content.Text)
	}

	if msg.Refusal != "" {
		c.Refusal(msg.Refusal)
	}

	done := true
	for _, toolCall := range msg.ToolCalls {
		done = false

		c.NotifyToolCall(toolCall.Function.Name, toolCall.Function.Arguments)
		result := a.executeTool(c, toolCall)
		c.NotifyToolCallResult(result.String())

		a.request.Messages = append(a.request.Messages, openrouter.ToolMessage(toolCall.ID, result.String()))
	}

	if !done {
		return nil
	}

	return errDone
}

func (a *agentImpl) executeTool(c iface.Context, toolCall openrouter.ToolCall) tools.ToolResult {
	if !a.canExecuteTool(c, toolCall) {
		return tools.ToolResult{
			Error: fmt.Errorf("tool %q is not available for this agent", toolCall.Function.Name),
		}
	}

	result := tools.Execute(c, toolCall.Function.Name, toolCall.Function.Arguments)
	return result
}

func (a *agentImpl) canExecuteTool(c iface.Context, toolCall openrouter.ToolCall) bool {
	for _, t := range a.tools {
		if t.Function.Name == toolCall.Function.Name {
			return true
		}
	}
	return false
}
