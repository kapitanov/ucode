package runner

import (
	"strings"

	"github.com/kapitanov/ucode/internal/iface"
)

func RunAgent(sandbox iface.Sandbox, agents iface.Agents, ui iface.UI, agent iface.Agent) error {
	ctx := &contextImpl{
		sandbox:   sandbox,
		agents:    agents,
		ui:        ui,
		plan:      newPersistedPlanStorage(),
		memory:    newMemoryStorage(),
		agentName: agent.Name(),
	}
	ui.SetPlan(ctx.Plan())
	ui.SetMemory(ctx.Memory())
	return agent.Run(ctx)
}

type contextImpl struct {
	agentName        string
	output           strings.Builder
	agents           iface.Agents
	sandbox          iface.Sandbox
	ui               iface.UI
	plan             planStorage
	memory           *memoryStorage
	inputStream      []string
	shouldPrintUsage bool
}

// Input
func (c *contextImpl) UI() iface.UI { return c.ui }

func (c *contextImpl) Prompt() string {
	if c.inputStream == nil {
		if c.shouldPrintUsage {
			c.ui.Usage()
		}
		c.shouldPrintUsage = true
		return c.ui.Prompt()
	}

	if len(c.inputStream) == 0 {
		return ""
	}

	str := c.inputStream[0]
	c.inputStream = c.inputStream[1:]

	return str
}

func (c *contextImpl) Ask(question string, options []string) int { return c.ui.Ask(question, options) }
func (c *contextImpl) Thinking() iface.ThinkingToken             { return c.ui.Thinking() }

// Output
func (c *contextImpl) Response(response string) {
	_, _ = c.output.WriteString(response)
	_, _ = c.output.WriteString("\n")
	c.ui.Response(c.agentName, response)
}
func (c *contextImpl) Reasoning(response string) { c.ui.Reasoning(c.agentName, response) }
func (c *contextImpl) Refusal(response string)   { c.ui.Refusal(c.agentName, response) }
func (c *contextImpl) ToolCall(toolCall iface.ToolCall) iface.ToolCallToken {
	return c.ui.ToolCall(toolCall)
}

func (c *contextImpl) Plan() *iface.Plan { return c.plan.Get() }
func (c *contextImpl) ClearPlan() {
	plan := c.plan.Get()
	plan.Items = []iface.PlanItem{}
	c.ui.SetPlan(plan)
	_ = c.plan.Save()
}
func (c *contextImpl) WritePlan(items []string) *iface.Plan {
	plan := c.plan.Get()
	for _, item := range items {
		plan.Items = append(plan.Items, iface.PlanItem{Index: len(plan.Items) + 1, Done: false, Title: item})
	}
	c.ui.SetPlan(plan)
	_ = c.plan.Save()
	return plan
}
func (c *contextImpl) CheckPlanItem(indices ...int) *iface.Plan {
	plan := c.plan.Get()
	for _, index := range indices {
		i := index - 1
		if i < 0 || i >= len(plan.Items) {
			continue
		}
		plan.Items[i].Done = true
	}

	c.ui.SetPlan(plan)
	_ = c.plan.Save()
	return plan
}

func (c *contextImpl) Memory() *iface.Memory { return c.memory.Get() }
func (c *contextImpl) WriteMemory(key, value string) *iface.Memory {
	memory := c.memory.Get()
	for i := range memory.Items {
		if memory.Items[i].Key == key {
			memory.Items[i].Value = value
			c.ui.SetMemory(memory)
			_ = c.memory.Save()
			return memory
		}
	}

	memory.Items = append(memory.Items, iface.MemoryItem{Key: key, Value: value})
	c.ui.SetMemory(memory)
	_ = c.memory.Save()
	return memory
}

func (c *contextImpl) Sandbox() iface.Sandbox { return c.sandbox }
func (c *contextImpl) Agents() iface.Agents   { return c.agents }

func (c *contextImpl) RunSubagent(agent iface.Agent, request string) (string, error) {
	_ = writeSubagentCallFile(agent, request)
	ctx := &contextImpl{
		sandbox:     c.sandbox,
		agents:      c.agents,
		ui:          c.ui,
		plan:        newTransientPlanStorage(),
		memory:      c.memory,
		agentName:   agent.Name(),
		inputStream: []string{request},
	}

	err := agent.Run(ctx)
	if err != nil {
		return "", err
	}

	output := ctx.output.String()
	return output, nil
}
