package agent

import (
	"context"
	_ "embed"
	"fmt"
	"time"

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
	Model        string
	ProviderType string

	client            LLMClient
	request           openrouter.ChatCompletionRequest
	maxMessages       int
	compactedMessages int
}

type Parameters struct {
	LLM               LLMClient
	ModelName         string
	MaxMessages       int
	CompactedMessages int
}

type LLMClient interface {
	Type() string
	CreateChatCompletion(ctx context.Context, req openrouter.ChatCompletionRequest) (*openrouter.ChatCompletionResponse, error)
}

func New(p Parameters) *Agent {
	request := openrouter.ChatCompletionRequest{
		Model: p.ModelName,
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage(prompt),
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
		Model:             p.ModelName,
		ProviderType:      p.LLM.Type(),
		client:            p.LLM,
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
	Usage        *Usage
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

type Usage struct {
	Tokens int
	Cost   float64
}

func (a *Agent) Run(str string) <-chan Event {
	ch := make(chan Event, 10)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("agent panic: %v\n", r)
			}
			close(ch)
		}()
		a.run(str, ch)
	}()

	return ch
}

func (a *Agent) run(str string, ch chan<- Event) {
	a.request.Messages = append(a.request.Messages, openrouter.UserMessage(str))
	for {
		response, done, err := a.runSingleOperation(ch)
		if err != nil {
			ch <- Event{Error: err}
			return
		}

		if done {
			if response != nil && response.Usage != nil {
				ch <- Event{Usage: &Usage{
					Tokens: response.Usage.TotalTokens,
					Cost:   response.Usage.Cost,
				}}
			}

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
		// No safe cut point found; force-compact by keeping the last compactedMessages entries
		// to prevent unbounded memory growth.
		keep := a.compactedMessages
		if keep >= len(a.request.Messages) {
			keep = len(a.request.Messages) - 1
		}
		recentMessages := a.request.Messages[len(a.request.Messages)-keep:]
		a.request.Messages = make([]openrouter.ChatCompletionMessage, 0, len(recentMessages)+1)
		a.request.Messages = append(a.request.Messages, systemMsg)
		a.request.Messages = append(a.request.Messages, recentMessages...)
		return &Compaction{Before: before, After: len(a.request.Messages)}
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

	for i := 2; i < len(messages); i++ {
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

func (a *Agent) runSingleOperation(ch chan<- Event) (*openrouter.ChatCompletionResponse, bool, error) {
	if compaction := a.compactMessages(); compaction != nil {
		ch <- Event{Compaction: compaction}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	response, err := a.client.CreateChatCompletion(ctx, a.request)
	if err != nil {
		return nil, false, err
	}

	if len(response.Choices) == 0 {
		return nil, false, fmt.Errorf("api returned empty choices")
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

	return response, done, nil
}
