package llm

import (
	"context"

	"github.com/revrost/go-openrouter"
)

type OpenRouterClient struct {
	client *openrouter.Client
	usage  openrouter.Usage
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

func (c *OpenRouterClient) Usage() openrouter.Usage { return c.usage }

func (c *OpenRouterClient) CreateChatCompletion(ctx context.Context, req openrouter.ChatCompletionRequest) (*openrouter.ChatCompletionResponse, error) {
	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Usage != nil {
		c.usage.Cost += resp.Usage.Cost
		c.usage.TotalTokens += resp.Usage.TotalTokens
		c.usage.PromptTokens += resp.Usage.PromptTokens
		c.usage.CompletionTokens += resp.Usage.CompletionTokens
	}
	return &resp, nil
}
