package llm

import (
	"context"

	"github.com/revrost/go-openrouter"
)

type OpenRouterClient struct {
	client *openrouter.Client
}

func NewOpenRouterClient(apiURL, apiKey string) *OpenRouterClient {
	configure := func(c *openrouter.ClientConfig) {
		if apiURL != "" {
			c.BaseURL = apiURL
		}
	}

	return &OpenRouterClient{
		client: openrouter.NewClient(apiKey, configure),
	}
}

func (*OpenRouterClient) Type() string { return "openrouter" }

func (c *OpenRouterClient) CreateChatCompletion(ctx context.Context, req openrouter.ChatCompletionRequest) (*openrouter.ChatCompletionResponse, error) {
	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
