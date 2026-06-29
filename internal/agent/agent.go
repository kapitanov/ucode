package agent

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/kapitanov/ucode/internal/tools"
	"github.com/revrost/go-openrouter"
)

const (
	DefaultModelName         = "anthropic/claude-haiku-4.5"
	DefaultMaxMessages       = 100
	DefaultCompactedMessages = 25
)

var (
	//go:embed PROMPT.md
	prompt string
)

type Agent struct {
	client            *openrouter.Client
	request           openrouter.ChatCompletionRequest
	maxMessages       int
	compactedMessages int
}

type Parameters struct {
	APIKey            string
	ModelName         string
	MaxMessages       int
	CompactedMessages int
}

func New(p Parameters) *Agent {
	request := openrouter.ChatCompletionRequest{
		Model: p.ModelName,
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage(prompt),
		},
		Reasoning: &openrouter.ChatCompletionReasoning{
			Enabled: new(false),
		},
		Tools: tools.Definitions(),
	}

	maxMessages := p.MaxMessages
	if maxMessages <= 0 {
		maxMessages = DefaultMaxMessages
	}

	compactedMessages := p.CompactedMessages
	if compactedMessages <= 0 {
		compactedMessages = DefaultCompactedMessages
	}

	return &Agent{
		client:            openrouter.NewClient(p.APIKey),
		request:           request,
		maxMessages:       maxMessages,
		compactedMessages: compactedMessages,
	}
}

type Event struct {
	Message      *Message
	Reasoning    *Reasoning
	ToolCall     *ToolCall
	ToolResponse *ToolResponse
	Compaction   *Compaction
	Error        error
}

type Compaction struct {
	Before, After int
}

type Message struct {
	Text string
}

type Reasoning struct {
	Text string
}

type ToolCall struct {
	Name, Args string
}

type ToolResponse struct {
	Name, Args    string
	Output, Error string
}

func (a *Agent) Run(str string) <-chan Event {
	ch := make(chan Event)

	go func() {
		defer close(ch)
		a.run(str, ch)
	}()

	return ch
}

func (a *Agent) run(str string, ch chan<- Event) {
	a.request.Messages = append(a.request.Messages, openrouter.UserMessage(str))
	for {
		done, err := a.runSingleOperation(ch)
		if err != nil {
			ch <- Event{Error: err}
			return
		}
		if done {
			return
		}
	}
}

func (a *Agent) compactMessages() *Compaction {
	if len(a.request.Messages) <= a.maxMessages {
		return nil
	}

	before := len(a.request.Messages)
	systemMsg := a.request.Messages[0]

	cutIndex := a.findSafeCutIndex()
	if cutIndex <= 1 || cutIndex >= len(a.request.Messages) {
		return nil
	}

	recentMessages := a.request.Messages[cutIndex:]

	a.request.Messages = make([]openrouter.ChatCompletionMessage, 0, len(recentMessages)+1)
	a.request.Messages = append(a.request.Messages, systemMsg)
	a.request.Messages = append(a.request.Messages, recentMessages...)

	return &Compaction{Before: before, After: len(a.request.Messages)}
}

func (a *Agent) findSafeCutIndex() int {
	messages := a.request.Messages
	targetIndex := len(messages) - a.compactedMessages

	if targetIndex <= 1 {
		targetIndex = 2
	}

	for i := targetIndex; i > 1; i-- {
		if a.isSafeCutPoint(i) {
			return i
		}
	}

	for i := targetIndex + 1; i < len(messages); i++ {
		if a.isSafeCutPoint(i) {
			return i
		}
	}

	return a.findFirstSafeCutIndex()
}

func (a *Agent) findFirstSafeCutIndex() int {
	for i := 2; i < len(a.request.Messages); i++ {
		if a.isSafeCutPoint(i) {
			return i
		}
	}
	return 0
}

func (a *Agent) isSafeCutPoint(index int) bool {
	if index <= 1 || index >= len(a.request.Messages) {
		return false
	}

	msg := a.request.Messages[index]

	if msg.Role == "tool" {
		return false
	}

	if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
		return false
	}

	return true
}

func (a *Agent) runSingleOperation(ch chan<- Event) (bool, error) {
	if compaction := a.compactMessages(); compaction != nil {
		ch <- Event{Compaction: compaction}
	}

	response, err := a.client.CreateChatCompletion(context.Background(), a.request)
	if err != nil {
		return false, err
	}

	if len(response.Choices) == 0 {
		return false, fmt.Errorf("api returned empty choices")
	}

	msg := response.Choices[0].Message
	a.request.Messages = append(a.request.Messages, msg)

	if msg.Reasoning != nil {
		ch <- Event{Reasoning: &Reasoning{Text: *msg.Reasoning}}
	}

	done := true
	for _, toolCall := range msg.ToolCalls {
		done = false
		ch <- Event{ToolCall: &ToolCall{Name: toolCall.Function.Name, Args: toolCall.Function.Arguments}}
		toolResponse, err := tools.Execute(toolCall.Function.Name, toolCall.Function.Arguments)
		if err != nil {
			toolResponse = fmt.Sprintf("ERROR: %v", err)
			ch <- Event{ToolResponse: &ToolResponse{Name: toolCall.Function.Name, Args: toolCall.Function.Arguments, Error: toolResponse}}
		} else {
			ch <- Event{ToolResponse: &ToolResponse{Name: toolCall.Function.Name, Args: toolCall.Function.Arguments, Output: toolResponse}}
		}

		a.request.Messages = append(a.request.Messages, openrouter.ToolMessage(toolCall.ID, toolResponse))
	}

	if msg.Content.Text != "" {
		ch <- Event{Message: &Message{Text: msg.Content.Text}}
	}

	if msg.Refusal != "" {
		ch <- Event{Error: fmt.Errorf("%s", msg.Refusal)}
	}

	return done, nil
}
