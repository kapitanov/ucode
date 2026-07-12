package agents

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/kapitanov/ucode/internal/agents/compaction"
	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tools"
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

	return &agentsImpl{
		agentsConfig: agentsConfig,
		llmConn:      llmConn,
		defaultModel: defaultModel,
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
	Prompt         string   `yaml:"prompt"`
	Model          string   `yaml:"model"`
	ForbiddenTools []string `yaml:"forbidden_tools"`
}

type agentsImpl struct {
	agentsConfig agentsConfigYAML
	llmConn      iface.LLM
	defaultModel string
}

func (a *agentsImpl) Default() iface.Agent {
	agent := a.ByRole(a.agentsConfig.DefaultRole)
	if agent == nil {
		panic("default agent role is not defined")
	}
	return agent
}

func (a *agentsImpl) ByRole(role string) iface.Agent {
	def, ok := a.agentsConfig.Roles[role]
	if !ok {
		return nil
	}

	prompt := def.Prompt + "\n\n" + a.agentsConfig.SharedPrompt
	model := def.Model
	if model == "" {
		model = a.defaultModel
		if model == "" {
			model = a.agentsConfig.DefaultModel
		}
	}

	agent := newAgentImpl(def.Name, prompt, model, def.ForbiddenTools, a.llmConn)
	return agent
}

type agentImpl struct {
	name, prompt, model string
	llm                 iface.LLM

	request           openrouter.ChatCompletionRequest
	maxMessages       int
	compactedMessages int
}

func newAgentImpl(name, prompt, model string, forbiddenTools []string, llmConn iface.LLM) *agentImpl {
	var ts []openrouter.Tool
	for _, t := range tools.All() {
		if slices.Contains(forbiddenTools, t.Name) {
			continue
		}

		ts = append(ts, t.Definition)
	}

	return &agentImpl{
		name:   name,
		prompt: prompt,
		model:  model,
		llm:    llmConn,
		request: openrouter.ChatCompletionRequest{
			Model: model,
			Messages: []openrouter.ChatCompletionMessage{
				openrouter.SystemMessage(prompt),
			},
			Tools: ts,
		},
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

		result := tools.Execute(c, toolCall.Function.Name, toolCall.Function.Arguments)

		a.request.Messages = append(a.request.Messages, openrouter.ToolMessage(toolCall.ID, result.String()))
	}

	if !done {
		return nil
	}

	return errDone
}
