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
	"github.com/kapitanov/ucode/internal/agents/definitions"
	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tools"
	"github.com/kapitanov/ucode/internal/tui"
	"github.com/revrost/go-openrouter"
)

const (
	DefaultMaxMessages       = 100
	DefaultCompactedMessages = 25
)

func New(llmConn iface.LLM, defaultModel string) iface.Agents {
	a := &agentsImpl{
		llmConn:      llmConn,
		defaultModel: defaultModel,
		defaultAgent: nil,
		agents:       make(map[string]iface.Agent),
	}

	a.defaultAgent = a.registerAgent(definitions.TeamLead)
	a.registerAgent(definitions.Developer)
	a.registerAgent(definitions.Tester)
	a.registerAgent(definitions.DevOps)
	a.registerAgent(definitions.Architect)
	a.registerAgent(definitions.Security)
	a.registerAgent(definitions.Documentation)
	a.registerAgent(definitions.Researcher)
	a.registerAgent(definitions.Reviewer)

	return a
}

func (a *agentsImpl) registerAgent(def definitions.Agent) iface.Agent {
	agent := newAgentImpl(def.Name, def.Prompt, a.defaultModel, def.Tools, a.llmConn)
	a.agents[def.Name] = agent

	var toolsList []string
	for _, t := range agent.tools {
		toolsList = append(toolsList, t.Function.Name)
	}
	tui.Printf("%% Agent %q (%s):", agent.name, def.Role)
	tui.Printf("%%   Model = %s", a.defaultModel)
	tui.Printf("%%   Tools = %s", strings.Join(toolsList, ", "))

	return agent
}

type agentsImpl struct {
	llmConn      iface.LLM
	defaultModel string
	defaultAgent iface.Agent
	agents       map[string]iface.Agent
}

func (a *agentsImpl) Default() iface.Agent           { return a.defaultAgent }
func (a *agentsImpl) Select(name string) iface.Agent { return a.agents[name] }

type agentImpl struct {
	name          string
	prompt, model string
	llm           iface.LLM
	tools         []openrouter.Tool

	request           openrouter.ChatCompletionRequest
	maxMessages       int
	compactedMessages int
}

func newAgentImpl(name, prompt, model string, allowedTools []tools.Name, llmConn iface.LLM) *agentImpl {
	var ts []openrouter.Tool
	for _, t := range tools.All() {
		if !slices.Contains(allowedTools, t.Name) {
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
	// Must comfortably cover the LLM client's own retry/backoff waits (rate-limit and
	// empty-choices retries), or a legitimate wait gets cut off by ctx.Done() and
	// surfaces as "context deadline exceeded" instead of a real response.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	thinking := c.Thinking()
	response, err := a.llm.CreateChatCompletion(ctx, a.request, c.UI())
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
		text := *msg.Reasoning
		text = strings.Trim(text, " \n\r\t")
		c.Reasoning(text)
	}

	if msg.Content.Text != "" {
		text := msg.Content.Text
		text = strings.Trim(text, " \n\r\t")
		if text != "" {
			c.Response(text)
		}
	}

	if msg.Refusal != "" {
		text := msg.Refusal
		text = strings.Trim(text, " \n\r\t")
		c.Refusal(text)
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

	result := tools.Execute(c, tools.Name(toolCall.Function.Name), toolCall.Function.Arguments)
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
