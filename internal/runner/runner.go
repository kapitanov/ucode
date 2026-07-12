package runner

import (
	"strings"

	"github.com/kapitanov/ucode/internal/iface"
)

func RunAgent(agents iface.Agents, ui iface.UI, agent iface.Agent) error {
	ctx := newContext(agents, ui)
	ctx.agentName = agent.Name()
	return agent.Run(ctx)
}

type contextImpl struct {
	agentName   string
	output      strings.Builder
	agents      iface.Agents
	ui          iface.UI
	plan        iface.Plan
	memory      *iface.Memory
	inputStream []string
}

func newContext(agents iface.Agents, ui iface.UI) *contextImpl {
	return &contextImpl{
		agents: agents,
		ui:     ui,
		plan:   iface.Plan{},
		memory: &iface.Memory{},
	}
}

// Input
func (c *contextImpl) Prompt() string {
	if c.inputStream == nil {
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

func (c *contextImpl) Plan() iface.Plan { return c.plan }
func (c *contextImpl) ClearPlan() {
	c.plan = iface.Plan{}
	c.ui.SetPlan(c.plan)
}
func (c *contextImpl) WritePlan(items []string) iface.Plan {
	plan := c.plan
	for _, item := range items {
		plan.Items = append(plan.Items, iface.PlanItem{Index: len(plan.Items) + 1, Done: false, Title: item})
	}
	c.plan = plan
	c.ui.SetPlan(c.plan)
	return c.plan
}
func (c *contextImpl) CheckPlanItem(index int) iface.Plan {
	i := index - 1
	if i < 0 || i >= len(c.plan.Items) {
		return c.plan
	}
	c.plan.Items[i].Done = true
	c.ui.SetPlan(c.plan)
	return c.plan
}

func (c *contextImpl) Memory() iface.Memory { return *c.memory }
func (c *contextImpl) WriteMemory(key, value string) iface.Memory {
	for i := range c.memory.Items {
		if c.memory.Items[i].Key == key {
			c.memory.Items[i].Value = value
			c.ui.SetMemory(c.memory)
			return *c.memory
		}
	}

	c.memory.Items = append(c.memory.Items, iface.MemoryItem{Key: key, Value: value})
	c.ui.SetMemory(c.memory)
	return *c.memory
}

func (c *contextImpl) Agents() iface.Agents { return c.agents }

func (c *contextImpl) RunSubagent(agent iface.Agent, request string) (string, error) {
	ctx := newContext(c.agents, c.ui)
	ctx.agentName = agent.Name()
	ctx.memory = c.memory
	ctx.inputStream = []string{request}

	err := agent.Run(ctx)
	if err != nil {
		return "", err
	}

	output := ctx.output.String()
	return output, nil
}
