package audios

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"vocabreview/backend/internal/domain"
)

type OpenAISpeechClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type speechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format"`
	Speed          float64 `json:"speed"`
}

func NewOpenAISpeechClient(baseURL, apiKey string, client *http.Client) *OpenAISpeechClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &OpenAISpeechClient{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, client: client}
}

func (c *OpenAISpeechClient) GenerateSpeech(ctx context.Context, job domain.VocabAudioJob) ([]byte, error) {
	body, err := json.Marshal(speechRequest{
		Model:          job.Model,
		Input:          job.InputText,
		Voice:          job.Voice,
		ResponseFormat: job.OutputFormat,
		Speed:          job.Speed,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAI speech request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create OpenAI speech request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post OpenAI speech: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("OpenAI speech status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read OpenAI speech response: %w", err)
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("OpenAI speech response was empty")
	}
	return audio, nil
}
