package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tui"
	"github.com/revrost/go-openrouter"
)

const maxRateLimitRetries = 3

// fallbackRateLimitWait is used when X-RateLimit-Reset is absent or unparseable.
const fallbackRateLimitWait = 60 * time.Second

// emptyChoicesRetryWait is used before retrying a response that came back with no choices
// (observed with some providers/free models on transient upstream hiccups).
const emptyChoicesRetryWait = 3 * time.Second

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

func (c *OpenRouterClient) CreateChatCompletion(ctx context.Context, req openrouter.ChatCompletionRequest, ui iface.UI) (*openrouter.ChatCompletionResponse, error) {
	for attempt := range maxRateLimitRetries {
		resp, err := c.client.CreateChatCompletion(ctx, req)
		if err != nil {
			var apiErr *openrouter.APIError
			if errors.As(err, &apiErr) && apiErr.HTTPStatusCode == http.StatusTooManyRequests {
				if attempt == maxRateLimitRetries-1 {
					return nil, fmt.Errorf("rate limit exceeded after %d retries: %w", maxRateLimitRetries, err)
				}

				sleepDuration := timeUntilReset(apiErr, fallbackRateLimitWait)
				ui.RateLimit(sleepDuration)

				if sleepErr := sleepUntilReset(ctx, sleepDuration); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			return nil, err
		}

		if len(resp.Choices) == 0 {
			if attempt == maxRateLimitRetries-1 {
				return nil, fmt.Errorf("api returned empty choices after %d retries", maxRateLimitRetries)
			}

			tui.Printf("%% API returned empty choices, retrying (attempt %d/%d)...", attempt+1, maxRateLimitRetries)
			if sleepErr := sleepUntilReset(ctx, emptyChoicesRetryWait); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		if resp.Usage != nil {
			c.usage.Cost += resp.Usage.Cost
			c.usage.TotalTokens += resp.Usage.TotalTokens
			c.usage.PromptTokens += resp.Usage.PromptTokens
			c.usage.CompletionTokens += resp.Usage.CompletionTokens
		}
		return &resp, nil
	}

	// unreachable, but satisfies the compiler
	return nil, fmt.Errorf("rate limit exceeded after %d retries", maxRateLimitRetries)
}

func timeUntilReset(apiErr *openrouter.APIError, fallback time.Duration) time.Duration {
	d := fallback
	if apiErr.Metadata != nil {
		if headers, ok := (*apiErr.Metadata)["headers"].(map[string]any); ok {
			if resetRaw, ok := headers["X-RateLimit-Reset"]; ok {
				var resetMs float64
				switch v := resetRaw.(type) {
				case float64:
					resetMs = v
				case int64:
					resetMs = float64(v)
				}
				if resetMs > 0 {
					resetAt := time.UnixMilli(int64(resetMs))
					if wait := time.Until(resetAt); wait > 0 {
						d = wait
					}
				}
			}
		}
	}
	return d
}

// sleepUntilReset waits until the X-RateLimit-Reset window (ms epoch) or falls back to a fixed duration.
func sleepUntilReset(ctx context.Context, sleepDuration time.Duration) error {
	select {
	case <-time.After(sleepDuration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
