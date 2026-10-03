package openrouter

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	audioSpeechSuffix         = "/audio/speech"
	audioTranscriptionsSuffix = "/audio/transcriptions"
)

// SpeechResponseFormat controls the audio container returned by the speech endpoint.
type SpeechResponseFormat string

const (
	SpeechResponseFormatMp3 SpeechResponseFormat = "mp3"
	SpeechResponseFormatPcm SpeechResponseFormat = "pcm"
)

// AudioProvider contains provider-specific passthrough options keyed by provider slug.
type AudioProvider struct {
	Options map[string]map[string]any `json:"options,omitempty"`
}

// SpeechRequest represents a request to the /audio/speech endpoint.
//
// API reference: https://openrouter.ai/docs/api/api-reference/speech/create-audio-speech
type SpeechRequest struct {
	// Model is the TTS model slug to use.
	Model string `json:"model"`
	// Input is the text to synthesize.
	Input string `json:"input"`
	// Voice is the provider-specific voice identifier.
	Voice string `json:"voice"`
	// ResponseFormat controls the returned audio format. Defaults to pcm when omitted.
	ResponseFormat SpeechResponseFormat `json:"response_format,omitempty"`
	// Speed controls playback speed for providers that support it.
	Speed float64 `json:"speed,omitempty"`
	// Provider contains provider-specific passthrough options.
	Provider *AudioProvider `json:"provider,omitempty"`
}

// SpeechResponse contains raw audio bytes and selected response headers from /audio/speech.
type SpeechResponse struct {
	Audio        []byte
	ContentType  string
	GenerationID string
}

// TranscriptionInputAudio is base64-encoded audio input for /audio/transcriptions.
type TranscriptionInputAudio struct {
	Data   string      `json:"data"`
	Format AudioFormat `json:"format"`
}

// TranscriptionRequest represents a request to the /audio/transcriptions endpoint.
//
// API reference: https://openrouter.ai/docs/api/api-reference/transcriptions/create-audio-transcriptions
type TranscriptionRequest struct {
	// Model is the STT model slug to use.
	Model string `json:"model"`
	// InputAudio is base64-encoded audio to transcribe.
	InputAudio TranscriptionInputAudio `json:"input_audio"`
	// Language is an optional ISO-639-1 language code. The API auto-detects language when omitted.
	Language string `json:"language,omitempty"`
	// Temperature controls sampling temperature for transcription. Use a pointer to send an explicit zero.
	Temperature *float64 `json:"temperature,omitempty"`
	// Provider contains provider-specific passthrough options.
	Provider *AudioProvider `json:"provider,omitempty"`
}

// TranscriptionUsage contains usage and billing details returned by /audio/transcriptions.
type TranscriptionUsage struct {
	Cost         float64 `json:"cost"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	Seconds      float64 `json:"seconds"`
	TotalTokens  int     `json:"total_tokens"`
}

// TranscriptionResponse represents the response from /audio/transcriptions.
type TranscriptionResponse struct {
	Text  string              `json:"text"`
	Usage *TranscriptionUsage `json:"usage,omitempty"`
}

// NewTranscriptionInputAudio base64-encodes raw audio bytes for a transcription request.
func NewTranscriptionInputAudio(audio []byte, format AudioFormat) TranscriptionInputAudio {
	return TranscriptionInputAudio{
		Data:   encodeAudio(audio),
		Format: format,
	}
}

// NewTranscriptionInputAudioFromFile reads an audio file and base64-encodes it for a transcription request.
func NewTranscriptionInputAudioFromFile(filePath string) (TranscriptionInputAudio, error) {
	audio, format, err := readAudioFile(filePath)
	if err != nil {
		return TranscriptionInputAudio{}, err
	}
	return NewTranscriptionInputAudio(audio, format), nil
}

// CreateSpeech synthesizes speech from text and returns the raw audio bytestream.
func (c *Client) CreateSpeech(ctx context.Context, request SpeechRequest) (SpeechResponse, error) {
	req, err := c.newRequest(
		ctx,
		http.MethodPost,
		c.fullURL(audioSpeechSuffix),
		withBody(request),
		withContentType("application/json; charset=utf-8"),
	)
	if err != nil {
		return SpeechResponse{}, err
	}
	req.Header.Set("Accept", "audio/*")

	res, err := c.config.HTTPClient.Do(req)
	if err != nil {
		return SpeechResponse{}, err
	}
	defer res.Body.Close()

	if isFailureStatusCode(res) {
		return SpeechResponse{}, c.handleErrorResp(res)
	}

	audio, err := io.ReadAll(res.Body)
	if err != nil {
		return SpeechResponse{}, err
	}

	return SpeechResponse{
		Audio:        audio,
		ContentType:  res.Header.Get("Content-Type"),
		GenerationID: res.Header.Get("X-Generation-Id"),
	}, nil
}

// CreateTranscription transcribes base64-encoded audio into text.
func (c *Client) CreateTranscription(
	ctx context.Context,
	request TranscriptionRequest,
) (TranscriptionResponse, error) {
	req, err := c.newRequest(
		ctx,
		http.MethodPost,
		c.fullURL(audioTranscriptionsSuffix),
		withBody(request),
	)
	if err != nil {
		return TranscriptionResponse{}, err
	}

	var response TranscriptionResponse
	if err := c.sendRequest(req, &response); err != nil {
		return TranscriptionResponse{}, err
	}

	return response, nil
}

func encodeAudio(audio []byte) string {
	return base64.StdEncoding.EncodeToString(audio)
}

func readAudioFile(filePath string) ([]byte, AudioFormat, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", err
	}

	format, err := audioFormatFromFilePath(filePath)
	if err != nil {
		return nil, "", err
	}

	return fileData, format, nil
}

func audioFormatFromFilePath(filePath string) (AudioFormat, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3":
		return AudioFormatMp3, nil
	case ".wav":
		return AudioFormatWav, nil
	case ".flac":
		return AudioFormatFlac, nil
	case ".opus":
		return AudioFormatOpus, nil
	case ".pcm16":
		return AudioFormatPcm16, nil
	case ".pcm24":
		return AudioFormatPcm24, nil
	case ".aiff", ".aif":
		return AudioFormatAiff, nil
	case ".aac":
		return AudioFormatAac, nil
	case ".ogg":
		return AudioFormatOgg, nil
	case ".m4a":
		return AudioFormatM4a, nil
	case ".webm":
		return AudioFormatWebm, nil
	default:
		return "", fmt.Errorf("unsupported audio format: %s", ext)
	}
}
