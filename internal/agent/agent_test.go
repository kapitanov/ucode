package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/revrost/go-openrouter"
)

// MockLLMClient - поддельный LLM клиент для тестов
type MockLLMClient struct {
	responses      []*openrouter.ChatCompletionResponse
	requestCount   int
	shouldError    bool
	errorMessage   string
	callCounter    int
}

func (m *MockLLMClient) Type() string {
	return "mock"
}

func (m *MockLLMClient) CreateChatCompletion(ctx context.Context, req openrouter.ChatCompletionRequest) (*openrouter.ChatCompletionResponse, error) {
	if m.shouldError {
		return nil, fmt.Errorf("%s", m.errorMessage)
	}

	if m.callCounter >= len(m.responses) {
		return nil, fmt.Errorf("not enough responses prepared, request %d", m.callCounter)
	}

	resp := m.responses[m.callCounter]
	m.callCounter++
	return resp, nil
}

func TestNewAgentDefaultValues(t *testing.T) {
	mockClient := &MockLLMClient{}
	params := Parameters{
		LLM:       mockClient,
		ModelName: "test-model",
	}

	agent := New(params)

	if agent.Model != "test-model" {
		t.Errorf("ожидался модель test-model, получилась %s", agent.Model)
	}

	if agent.ProviderType != "mock" {
		t.Errorf("ожидался провайдер mock, получилась %s", agent.ProviderType)
	}

	if agent.maxMessages != DefaultMaxMessages {
		t.Errorf("ожидалось maxMessages=%d, получилось %d", DefaultMaxMessages, agent.maxMessages)
	}

	if agent.compactedMessages != DefaultCompactedMessages {
		t.Errorf("ожидалось compactedMessages=%d, получилось %d", DefaultCompactedMessages, agent.compactedMessages)
	}

	// Проверяем что в запросе уже есть системное сообщение
	if len(agent.request.Messages) != 1 {
		t.Errorf("ожидалось 1 сообщение (система), получилось %d", len(agent.request.Messages))
	}

	if agent.request.Messages[0].Role != "system" {
		t.Errorf("первое сообщение должно быть system role, получилось %s", agent.request.Messages[0].Role)
	}
}

func TestNewAgentCustomValues(t *testing.T) {
	mockClient := &MockLLMClient{}
	params := Parameters{
		LLM:               mockClient,
		ModelName:         "custom-model",
		MaxMessages:       50,
		CompactedMessages: 15,
	}

	agent := New(params)

	if agent.maxMessages != 50 {
		t.Errorf("ожидалось maxMessages=50, получилось %d", agent.maxMessages)
	}

	if agent.compactedMessages != 15 {
		t.Errorf("ожидалось compactedMessages=15, получилось %d", agent.compactedMessages)
	}
}

func TestNewAgentInvalidMaxMessages(t *testing.T) {
	mockClient := &MockLLMClient{}
	params := Parameters{
		LLM:               mockClient,
		ModelName:         "test-model",
		MaxMessages:       0,
		CompactedMessages: 10,
	}

	agent := New(params)

	if agent.maxMessages != DefaultMaxMessages {
		t.Errorf("при MaxMessages=0 должно использоваться значение по умолчанию %d, получилось %d", DefaultMaxMessages, agent.maxMessages)
	}
}

func TestIsSafeCutPointBasic(t *testing.T) {
	mockClient := &MockLLMClient{}
	agent := New(Parameters{LLM: mockClient, ModelName: "test"})

	// Добавляем тестовые сообщения
	agent.request.Messages = []openrouter.ChatCompletionMessage{
		{Role: "system", Content: openrouter.Content{Text: "system prompt"}},
		{Role: "user", Content: openrouter.Content{Text: "привет"}},
		{Role: "assistant", Content: openrouter.Content{Text: "привет!"}},
		{Role: "user", Content: openrouter.Content{Text: "как дела?"}},
	}

	// Индекс 0 и 1 - не валидны (система и начало)
	if agent.isSafeCutPoint(0) {
		t.Error("индекс 0 не должен быть точкой отреза (система)")
	}

	if agent.isSafeCutPoint(1) {
		t.Error("индекс 1 не должен быть точкой отреза (граница)")
	}

	// Индекс 2 - assistant без tool calls, валидно
	if !agent.isSafeCutPoint(2) {
		t.Error("индекс 2 должен быть валидной точкой отреза")
	}

	// Индекс 3 - user, валидно
	if !agent.isSafeCutPoint(3) {
		t.Error("индекс 3 должен быть валидной точкой отреза")
	}

	// Индекс вне границ
	if agent.isSafeCutPoint(100) {
		t.Error("индекс 100 не должен быть валидной точкой отреза")
	}
}

func TestIsSafeCutPointWithToolCall(t *testing.T) {
	mockClient := &MockLLMClient{}
	agent := New(Parameters{LLM: mockClient, ModelName: "test"})

	// Создаём сообщение с tool call
	toolCall := openrouter.ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: openrouter.FunctionCall{
			Name:      "some_tool",
			Arguments: `{"key":"value"}`,
		},
	}

	agent.request.Messages = []openrouter.ChatCompletionMessage{
		{Role: "system", Content: openrouter.Content{Text: "system"}},
		{Role: "user", Content: openrouter.Content{Text: "хэй"}},
		{
			Role:      "assistant",
			Content:   openrouter.Content{Text: ""},
			ToolCalls: []openrouter.ToolCall{toolCall},
		},
	}

	// Assistant с tool calls не должен быть точкой отреза
	if agent.isSafeCutPoint(2) {
		t.Error("индекс 2 (assistant с tool calls) не должен быть точкой отреза")
	}
}

func TestIsSafeCutPointWithToolMessage(t *testing.T) {
	mockClient := &MockLLMClient{}
	agent := New(Parameters{LLM: mockClient, ModelName: "test"})

	agent.request.Messages = []openrouter.ChatCompletionMessage{
		{Role: "system", Content: openrouter.Content{Text: "system"}},
		{Role: "user", Content: openrouter.Content{Text: "хэй"}},
		{Role: "tool", Content: openrouter.Content{Text: "результат"}},
	}

	// Tool сообщение не должно быть точкой отреза
	if agent.isSafeCutPoint(2) {
		t.Error("индекс 2 (tool сообщение) не должен быть точкой отреза")
	}
}

func TestFindSafeCutIndex(t *testing.T) {
	mockClient := &MockLLMClient{}
	agent := New(Parameters{
		LLM:               mockClient,
		ModelName:         "test",
		CompactedMessages: 2,
	})

	// Создаём цепочку сообщений
	agent.request.Messages = []openrouter.ChatCompletionMessage{
		{Role: "system", Content: openrouter.Content{Text: "system"}},
		{Role: "user", Content: openrouter.Content{Text: "msg1"}},
		{Role: "assistant", Content: openrouter.Content{Text: "resp1"}},
		{Role: "user", Content: openrouter.Content{Text: "msg2"}},
		{Role: "assistant", Content: openrouter.Content{Text: "resp2"}},
	}

	cutIndex := agent.findSafeCutIndex()

	// Должны найти валидный индекс для отреза
	if cutIndex <= 1 {
		t.Errorf("индекс отреза должен быть > 1, получилось %d", cutIndex)
	}

	if cutIndex >= len(agent.request.Messages) {
		t.Errorf("индекс отреза должен быть < %d, получилось %d", len(agent.request.Messages), cutIndex)
	}

	// Проверяем что это безопасная точка
	if !agent.isSafeCutPoint(cutIndex) {
		t.Errorf("найденный индекс %d не является безопасной точкой отреза", cutIndex)
	}
}

func TestCompactMessagesNoCompactionNeeded(t *testing.T) {
	mockClient := &MockLLMClient{}
	agent := New(Parameters{
		LLM:               mockClient,
		ModelName:         "test",
		MaxMessages:       10,
		CompactedMessages: 5,
	})

	// Добавляем меньше сообщений чем максимум
	for i := 0; i < 3; i++ {
		agent.request.Messages = append(agent.request.Messages,
			openrouter.UserMessage(fmt.Sprintf("msg%d", i)))
	}

	compaction := agent.compactMessages()

	// Не должна быть компакция
	if compaction != nil {
		t.Error("компакция не должна требоваться при количестве сообщений < maxMessages")
	}

	// Количество сообщений не должно измениться
	if len(agent.request.Messages) != 4 { // система + 3 юзер
		t.Errorf("ожидалось 4 сообщения, получилось %d", len(agent.request.Messages))
	}
}

func TestCompactMessagesCompactionTriggered(t *testing.T) {
	mockClient := &MockLLMClient{}
	agent := New(Parameters{
		LLM:               mockClient,
		ModelName:         "test",
		MaxMessages:       5,
		CompactedMessages: 2,
	})

	// Добавляем много сообщений
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			agent.request.Messages = append(agent.request.Messages,
				openrouter.UserMessage(fmt.Sprintf("msg%d", i)))
		} else {
			agent.request.Messages = append(agent.request.Messages,
				openrouter.AssistantMessage(fmt.Sprintf("resp%d", i)))
		}
	}

	before := len(agent.request.Messages)
	compaction := agent.compactMessages()

	// Должна быть компакция
	if compaction == nil {
		t.Error("компакция должна требоваться")
	}

	if compaction.Before != before {
		t.Errorf("Before не совпадает, ожидалось %d, получилось %d", before, compaction.Before)
	}

	after := len(agent.request.Messages)
	if after >= before {
		t.Errorf("после компакции количество должно уменьшиться, было %d, стало %d", before, after)
	}

	if after > agent.maxMessages+1 { // +1 за системное сообщение
		t.Errorf("после компакции должно быть <= %d сообщений, получилось %d", agent.maxMessages+1, after)
	}

	// Первое сообщение должно остаться системным
	if agent.request.Messages[0].Role != "system" {
		t.Error("первое сообщение должно остаться системным после компакции")
	}
}

func TestCompactMessagesPreservesSystemMessage(t *testing.T) {
	mockClient := &MockLLMClient{}
	agent := New(Parameters{
		LLM:               mockClient,
		ModelName:         "test",
		MaxMessages:       3,
		CompactedMessages: 2,
	})

	systemMsg := agent.request.Messages[0]

	// Добавляем много сообщений
	for i := 0; i < 10; i++ {
		agent.request.Messages = append(agent.request.Messages,
			openrouter.UserMessage(fmt.Sprintf("msg%d", i)))
	}

	agent.compactMessages()

	// Проверяем что системное сообщение осталось
	if len(agent.request.Messages) == 0 {
		t.Fatal("системное сообщение было потеряно")
	}

	if agent.request.Messages[0].Role != "system" {
		t.Error("первое сообщение должно быть system role")
	}

	if agent.request.Messages[0].Content.Text != systemMsg.Content.Text {
		t.Error("содержание системного сообщения изменилось")
	}
}

func TestRunSingleMessage(t *testing.T) {
	mockClient := &MockLLMClient{
		responses: []*openrouter.ChatCompletionResponse{
			{
				Choices: []openrouter.ChatCompletionChoice{
					{
						Message: openrouter.ChatCompletionMessage{
							Role: "assistant",
							Content: openrouter.Content{
								Text: "Hello from LLM!",
							},
						},
					},
				},
				Usage: &openrouter.Usage{
					TotalTokens: 100,
					Cost:        0.05,
				},
			},
		},
	}

	agent := New(Parameters{
		LLM:       mockClient,
		ModelName: "test",
	})

	events := []Event{}
	for event := range agent.Run("привет, как дела?") {
		events = append(events, event)
	}

	// Ищем сообщение в событиях
	hasMessage := false
	hasUsage := false

	for _, ev := range events {
		if ev.Message != nil && ev.Message.Text == "Hello from LLM!" {
			hasMessage = true
		}
		if ev.Usage != nil && ev.Usage.Tokens == 100 {
			hasUsage = true
		}
	}

	if !hasMessage {
		t.Error("ожидалось событие Message в результатах")
	}

	if !hasUsage {
		t.Error("ожидалось событие Usage в результатах")
	}

	if len(events) < 2 {
		t.Errorf("ожидалось минимум 2 события, получилось %d", len(events))
	}
}

func TestRunWithToolCall(t *testing.T) {
	mockClient := &MockLLMClient{
		responses: []*openrouter.ChatCompletionResponse{
			{
				Choices: []openrouter.ChatCompletionChoice{
					{
						Message: openrouter.ChatCompletionMessage{
							Role: "assistant",
							Content: openrouter.Content{
								Text: "",
							},
							ToolCalls: []openrouter.ToolCall{
								{
									ID:   "call_123",
									Type: "function",
									Function: openrouter.FunctionCall{
										Name:      "test_tool",
										Arguments: `{"param":"value"}`,
									},
								},
							},
						},
					},
				},
				Usage: &openrouter.Usage{
					TotalTokens: 50,
					Cost:        0.02,
				},
			},
			{
				Choices: []openrouter.ChatCompletionChoice{
					{
						Message: openrouter.ChatCompletionMessage{
							Role: "assistant",
							Content: openrouter.Content{
								Text: "Done!",
							},
						},
					},
				},
				Usage: &openrouter.Usage{
					TotalTokens: 75,
					Cost:        0.03,
				},
			},
		},
	}

	agent := New(Parameters{
		LLM:       mockClient,
		ModelName: "test",
	})

	events := []Event{}
	for event := range agent.Run("запусти инструмент") {
		events = append(events, event)
	}

	hasToolCall := false
	hasToolResponse := false
	hasMessage := false

	for _, ev := range events {
		if ev.ToolCall != nil && ev.ToolCall.Name == "test_tool" {
			hasToolCall = true
		}
		if ev.ToolResponse != nil && ev.ToolResponse.Name == "test_tool" {
			hasToolResponse = true
		}
		if ev.Message != nil && ev.Message.Text == "Done!" {
			hasMessage = true
		}
	}

	if !hasToolCall {
		t.Error("ожидалось событие ToolCall в результатах")
	}

	if !hasToolResponse {
		t.Error("ожидалось событие ToolResponse в результатах")
	}

	if !hasMessage {
		t.Error("ожидалось финальное сообщение в результатах")
	}
}

func TestRunWithError(t *testing.T) {
	mockClient := &MockLLMClient{
		shouldError:  true,
		errorMessage: "LLM service is down",
	}

	agent := New(Parameters{
		LLM:       mockClient,
		ModelName: "test",
	})

	events := []Event{}
	for event := range agent.Run("привет") {
		events = append(events, event)
	}

	// Должно быть событие об ошибке
	hasError := false
	for _, ev := range events {
		if ev.Error != nil {
			hasError = true
			break
		}
	}

	if !hasError {
		t.Error("ожидалось событие Error в результатах")
	}
}

func TestRunWithEmptyResponse(t *testing.T) {
	mockClient := &MockLLMClient{
		responses: []*openrouter.ChatCompletionResponse{
			{
				Choices: []openrouter.ChatCompletionChoice{}, // Пустой ответ
			},
		},
	}

	agent := New(Parameters{
		LLM:       mockClient,
		ModelName: "test",
	})

	events := []Event{}
	for event := range agent.Run("привет") {
		events = append(events, event)
	}

	// Должно быть событие об ошибке (пустые choices)
	hasError := false
	for _, ev := range events {
		if ev.Error != nil {
			hasError = true
			break
		}
	}

	if !hasError {
		t.Error("ожидалось событие Error для пустого ответа")
	}
}

func TestCompactMessagesEdgeCaseAllToolMessages(t *testing.T) {
	mockClient := &MockLLMClient{}
	agent := New(Parameters{
		LLM:               mockClient,
		ModelName:         "test",
		MaxMessages:       2,
		CompactedMessages: 1,
	})

	// Добавляем только tool сообщения (небезопасные точки отреза)
	agent.request.Messages = []openrouter.ChatCompletionMessage{
		{Role: "system", Content: openrouter.Content{Text: "system"}},
		{Role: "tool", Content: openrouter.Content{Text: "tool1"}},
		{Role: "tool", Content: openrouter.Content{Text: "tool2"}},
		{Role: "tool", Content: openrouter.Content{Text: "tool3"}},
		{Role: "tool", Content: openrouter.Content{Text: "tool4"}},
	}

	before := len(agent.request.Messages)
	compaction := agent.compactMessages()

	// Должна быть компакция даже если нет безопасных точек
	if compaction == nil {
		t.Fatal("компакция должна произойти")
	}

	after := len(agent.request.Messages)

	// Система должна остаться
	if agent.request.Messages[0].Role != "system" {
		t.Error("система должна остаться первой")
	}

	// Количество должно уменьшиться
	if after >= before {
		t.Errorf("количество должно уменьшиться, было %d, стало %d", before, after)
	}

	// Должны остаться последние сообщения и система
	if after != agent.compactedMessages+1 { // +1 за систему
		t.Errorf("ожидалось %d сообщений, получилось %d", agent.compactedMessages+1, after)
	}
}

func TestRunMessagesAccumulate(t *testing.T) {
	mockClient := &MockLLMClient{
		responses: []*openrouter.ChatCompletionResponse{
			{
				Choices: []openrouter.ChatCompletionChoice{
					{
						Message: openrouter.ChatCompletionMessage{
							Role: "assistant",
							Content: openrouter.Content{
								Text: "first response",
							},
						},
					},
				},
				Usage: &openrouter.Usage{
					TotalTokens: 10,
					Cost:        0.01,
				},
			},
		},
	}

	agent := New(Parameters{
		LLM:       mockClient,
		ModelName: "test",
	})

	initialCount := len(agent.request.Messages) // Should be 1 (system)

	// Consume all events to ensure goroutine completes
	for range agent.Run("hello") {
	}

	finalCount := len(agent.request.Messages)

	// Messages should accumulate after run
	if finalCount <= initialCount {
		t.Errorf("messages should accumulate, was %d, now %d", initialCount, finalCount)
	}

	// Should have: system, user, assistant
	foundUser := false
	foundAssistant := false

	for _, msg := range agent.request.Messages {
		if msg.Role == "user" {
			foundUser = true
		}
		if msg.Role == "assistant" {
			foundAssistant = true
		}
	}

	if !foundUser {
		t.Error("history should contain user message")
	}

	if !foundAssistant {
		t.Error("history should contain assistant response")
	}
}
