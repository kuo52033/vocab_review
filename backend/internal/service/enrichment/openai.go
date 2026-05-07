package enrichment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OpenAIProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

type openAIChatRequest struct {
	Model          string            `json:"model"`
	Messages       []openAIMessage   `json:"messages"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []openAIChoice `json:"choices"`
}

type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

func NewOpenAIProvider(baseURL, apiKey, model string, client *http.Client) *OpenAIProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &OpenAIProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		client:  client,
	}
}

func (p *OpenAIProvider) Complete(ctx context.Context, items []Item) ([]Suggestion, error) {
	itemJSON, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("marshal enrichment items: %w", err)
	}

	chatReq := openAIChatRequest{
		Model: p.model,
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: "Return strict JSON only. Allowed part_of_speech values: noun, verb, adjective, adverb, phrase, idiom, phrasal_verb, preposition, conjunction, interjection, determiner, pronoun, other.",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Fill missing vocabulary details for these items, preserving order: %s\nReturn JSON exactly matching this shape: {\"items\":[{\"term\":\"\",\"meaning\":\"\",\"example_sentence\":\"\",\"part_of_speech\":\"\",\"error\":\"\"}]}", itemJSON),
			},
		},
		ResponseFormat: map[string]string{"type": "json_object"},
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAI chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create OpenAI chat request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post OpenAI chat completions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("OpenAI chat completions status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode OpenAI chat response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI chat response contained no choices")
	}

	var parsed struct {
		Items []Suggestion `json:"items"`
	}
	if err := json.Unmarshal([]byte(chatResp.Choices[0].Message.Content), &parsed); err != nil {
		return nil, fmt.Errorf("decode OpenAI chat content JSON: %w", err)
	}

	return parsed.Items, nil
}
