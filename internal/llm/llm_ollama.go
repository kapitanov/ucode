package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/revrost/go-openrouter"
)

type OllamaClient struct {
	baseURL string
	client  *http.Client
}

func NewOllamaClient(baseURL string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{},
	}
}

func (c *OllamaClient) CreateChatCompletion(ctx context.Context, req openrouter.ChatCompletionRequest) (*openrouter.ChatCompletionResponse, error) {
	messages := make([]map[string]interface{}, len(req.Messages))
	for i, msg := range req.Messages {
		m := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content.Text,
		}
		messages[i] = m
	}

	ollamaReq := map[string]interface{}{
		"model":    req.Model,
		"messages": messages,
		"stream":   false,
	}

	if len(req.Tools) > 0 {
		toolsList := make([]map[string]interface{}, len(req.Tools))
		for i, tool := range req.Tools {
			toolsList[i] = map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        tool.Function.Name,
					"description": tool.Function.Description,
					"parameters":  tool.Function.Parameters,
				},
			}
		}
		ollamaReq["tools"] = toolsList
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call ollama: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var ollamaResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode ollama response: %w", err)
	}

	content := ""
	var reasoning *string
	var toolCalls []openrouter.ToolCall

	if msg, ok := ollamaResp["message"].(map[string]interface{}); ok {
		if text, ok := msg["content"].(string); ok {
			content = text
		}
		if r, ok := msg["reasoning"].(string); ok && r != "" {
			reasoning = &r
		}

		if toolCallsRaw, ok := msg["tool_calls"].([]interface{}); ok {
			for _, tcRaw := range toolCallsRaw {
				if tc, ok := tcRaw.(map[string]interface{}); ok {
					if fnRaw, ok := tc["function"].(map[string]interface{}); ok {
						name, _ := fnRaw["name"].(string)
						var args string
						switch v := fnRaw["arguments"].(type) {
						case string:
							args = v
						case map[string]interface{}:
							if b, err := json.Marshal(v); err == nil {
								args = string(b)
							}
						}
						id, _ := tc["id"].(string)
						if id == "" {
							id = name
						}

						toolCalls = append(toolCalls, openrouter.ToolCall{
							ID:   id,
							Type: openrouter.ToolTypeFunction,
							Function: openrouter.FunctionCall{
								Name:      name,
								Arguments: args,
							},
						})
					}
				}
			}
		}
	}

	return &openrouter.ChatCompletionResponse{
		Choices: []openrouter.ChatCompletionChoice{
			{
				Message: openrouter.ChatCompletionMessage{
					Role: "assistant",
					Content: openrouter.Content{
						Text: content,
					},
					ToolCalls: toolCalls,
					Reasoning: reasoning,
				},
			},
		},
	}, nil
}
