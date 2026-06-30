package llm

import (
	"context"
	"encoding/json"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
	"github.com/revrost/go-openrouter"
)

type OpenAIClient struct {
	client openai.Client
}

func NewOpenAIClient(apiURL, apiKey string) *OpenAIClient {
	options := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if apiURL != "" {
		options = append(options, option.WithBaseURL(apiURL))
	}

	client := openai.NewClient(options...)
	return &OpenAIClient{
		client: client,
	}
}

func (c *OpenAIClient) Type() string { return "openai" }

func (c *OpenAIClient) CreateChatCompletion(ctx context.Context, req openrouter.ChatCompletionRequest) (*openrouter.ChatCompletionResponse, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, len(req.Messages))
	for i, msg := range req.Messages {
		content := msg.Content.Text
		switch msg.Role {
		case "system":
			messages[i] = openai.SystemMessage(content)
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, len(msg.ToolCalls))
				for j, tc := range msg.ToolCalls {
					toolCalls[j] = openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID: tc.ID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Function.Name,
								Arguments: tc.Function.Arguments,
							},
						},
					}
				}
				assistant := openai.ChatCompletionAssistantMessageParam{
					ToolCalls: toolCalls,
				}
				if content != "" {
					assistant.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(content),
					}
				}
				messages[i] = openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant}
			} else {
				messages[i] = openai.AssistantMessage(content)
			}
		case "tool":
			messages[i] = openai.ToolMessage(content, msg.ToolCallID)
		default:
			messages[i] = openai.UserMessage(content)
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    req.Model,
		Messages: messages,
	}
	if req.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(req.MaxTokens))
	}
	if req.Temperature > 0 {
		params.Temperature = openai.Float(float64(req.Temperature))
	}
	if req.FrequencyPenalty > 0 {
		params.FrequencyPenalty = openai.Float(float64(req.FrequencyPenalty))
	}
	if req.PresencePenalty > 0 {
		params.PresencePenalty = openai.Float(float64(req.PresencePenalty))
	}

	if len(req.Tools) > 0 {
		tools := make([]openai.ChatCompletionToolUnionParam, len(req.Tools))
		for i, tool := range req.Tools {
			fn := shared.FunctionDefinitionParam{
				Name: tool.Function.Name,
			}
			if tool.Function.Description != "" {
				fn.Description = openai.String(tool.Function.Description)
			}
			if tool.Function.Parameters != nil {
				if p, ok := tool.Function.Parameters.(map[string]any); ok {
					fn.Parameters = shared.FunctionParameters(p)
				}
			}
			tools[i] = openai.ChatCompletionToolUnionParam{
				OfFunction: &openai.ChatCompletionFunctionToolParam{Function: fn},
			}
		}
		params.Tools = tools
	}

	if req.ToolChoice != nil {
		if v, ok := req.ToolChoice.(string); ok {
			params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
				OfAuto: openai.String(v),
			}
		}
	}

	resp, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, err
	}

	choices := make([]openrouter.ChatCompletionChoice, len(resp.Choices))
	for i, choice := range resp.Choices {
		var toolCalls []openrouter.ToolCall
		for _, tc := range choice.Message.ToolCalls {
			fn := tc.AsFunction()
			toolCalls = append(toolCalls, openrouter.ToolCall{
				ID:   fn.ID,
				Type: openrouter.ToolTypeFunction,
				Function: openrouter.FunctionCall{
					Name:      fn.Function.Name,
					Arguments: fn.Function.Arguments,
				},
			})
		}

		var reasoning *string
		if field, ok := choice.Message.JSON.ExtraFields["reasoning"]; ok && field.Valid() {
			var text string
			if err := json.Unmarshal([]byte(field.Raw()), &text); err == nil && text != "" {
				reasoning = &text
			}
		}

		choices[i] = openrouter.ChatCompletionChoice{
			Index: int(choice.Index),
			Message: openrouter.ChatCompletionMessage{
				Role: "assistant",
				Content: openrouter.Content{
					Text: choice.Message.Content,
				},
				ToolCalls: toolCalls,
				Reasoning: reasoning,
			},
			FinishReason: openrouter.FinishReason(choice.FinishReason),
		}
	}

	return &openrouter.ChatCompletionResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: resp.Created,
		Model:   resp.Model,
		Choices: choices,
	}, nil
}
