package openrouter

// UserMessageWithAudioFromFile creates a user message with the given prompt text and audio file.
// It reads the audio file and creates a message with the embedded audio data.
func UserMessageWithAudioFromFile(promptText, filePath string) (ChatCompletionMessage, error) {
	fileData, format, err := readAudioFile(filePath)
	if err != nil {
		return ChatCompletionMessage{}, err
	}

	msg := UserMessageWithAudio(promptText, fileData, format)

	return msg, nil
}

// UserMessageWithAudio creates a user message with the given prompt text and audio content.
// Creates a message with the embedded audio data.
func UserMessageWithAudio(promptText string, audio []byte, format AudioFormat) ChatCompletionMessage {
	msg := ChatCompletionMessage{
		Role: ChatMessageRoleUser,
		Content: Content{
			Multi: []ChatMessagePart{
				{
					Type: ChatMessagePartTypeText,
					Text: promptText,
				},
				chatMessagePartWithAudio(audio, format),
			},
		},
	}

	return msg
}

// chatMessagePartWithAudio creates a ChatMessagePart which contains the given audio content.
func chatMessagePartWithAudio(audio []byte, format AudioFormat) ChatMessagePart {
	msg := ChatMessagePart{
		Type: ChatMessagePartTypeInputAudio,
		InputAudio: &ChatMessageInputAudio{
			Format: format,
			Data:   encodeAudio(audio),
		},
	}

	return msg
}
