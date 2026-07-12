package compaction

import "github.com/revrost/go-openrouter"

const (
	MaxMessages       = 100
	CompactedMessages = 25
)

type Result struct {
	Before, After int
	Request       openrouter.ChatCompletionRequest
}

func Compact(request openrouter.ChatCompletionRequest) *Result {
	if len(request.Messages) <= MaxMessages {
		return nil
	}

	before := len(request.Messages)
	systemMsg := request.Messages[0]

	cutIndex := findSafeCutIndex(request)
	if cutIndex <= 1 || cutIndex >= len(request.Messages) {
		// No safe cut point found; force-compact by keeping the last CompactedMessages entries
		// to prevent unbounded memory growth.
		keep := CompactedMessages
		if keep >= len(request.Messages) {
			keep = len(request.Messages) - 1
		}
		recentMessages := request.Messages[len(request.Messages)-keep:]
		request.Messages = make([]openrouter.ChatCompletionMessage, 0, len(recentMessages)+1)
		request.Messages = append(request.Messages, systemMsg)
		request.Messages = append(request.Messages, recentMessages...)
		return &Result{
			Before:  before,
			After:   len(request.Messages),
			Request: request,
		}
	}

	recentMessages := request.Messages[cutIndex:]

	request.Messages = make([]openrouter.ChatCompletionMessage, 0, len(recentMessages)+1)
	request.Messages = append(request.Messages, systemMsg)
	request.Messages = append(request.Messages, recentMessages...)

	return &Result{
		Before:  before,
		After:   len(request.Messages),
		Request: request,
	}
}

func findSafeCutIndex(request openrouter.ChatCompletionRequest) int {
	messages := request.Messages
	targetIndex := len(messages) - CompactedMessages

	if targetIndex <= 1 {
		targetIndex = 2
	}

	for i := targetIndex; i > 1; i-- {
		if isSafeCutPoint(request, i) {
			return i
		}
	}

	for i := targetIndex + 1; i < len(messages); i++ {
		if isSafeCutPoint(request, i) {
			return i
		}
	}

	for i := 2; i < len(messages); i++ {
		if isSafeCutPoint(request, i) {
			return i
		}
	}
	return 0
}

func isSafeCutPoint(request openrouter.ChatCompletionRequest, index int) bool {
	if index <= 1 || index >= len(request.Messages) {
		return false
	}

	msg := request.Messages[index]

	if msg.Role == "tool" {
		return false
	}

	if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
		return false
	}

	return true
}
