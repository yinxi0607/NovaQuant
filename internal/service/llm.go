package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"NovaQuant/internal/config"
)

type LLMClient struct {
	baseURL    string
	apiKey     string
	model      string
	chatPath   string
	httpClient *http.Client
}

func NewLLMClient(cfg config.Config) *LLMClient {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.LLMBaseURL), "/")
	model := strings.TrimSpace(cfg.LLMModel)
	if baseURL == "" || model == "" {
		return nil
	}
	chatPath := strings.TrimSpace(cfg.LLMChatPath)
	if chatPath == "" {
		chatPath = "/v1/chat/completions"
	}
	if !strings.HasPrefix(chatPath, "/") {
		chatPath = "/" + chatPath
	}
	return &LLMClient{
		baseURL:    baseURL,
		apiKey:     strings.TrimSpace(cfg.LLMAPIKey),
		model:      model,
		chatPath:   chatPath,
		httpClient: &http.Client{Timeout: maxDuration(cfg.HTTPTimeout, 45*time.Second)},
	}
}

func (c *LLMClient) Enabled() bool {
	return c != nil && c.baseURL != "" && c.model != ""
}

func (c *LLMClient) Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if !c.Enabled() {
		return "", errors.New("llm is not configured")
	}
	body := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.35,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.chatPath, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("llm upstream %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if len(payload.Choices) == 0 {
		return "", errors.New("llm returned no choices")
	}
	return strings.TrimSpace(payload.Choices[0].Message.Content), nil
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
