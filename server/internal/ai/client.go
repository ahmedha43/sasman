package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"mikrotik-manager/server/internal/storage"
)

type LLMClient struct {
	httpClient *http.Client
}

func NewLLMClient() *LLMClient {
	return &LLMClient{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *LLMClient) Complete(ctx context.Context, settings *storage.AISettings, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	if settings == nil {
		return nil, fmt.Errorf("AI settings not configured")
	}

	apiKey := strings.TrimSpace(settings.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("لم يتم ضبط مفتاح الـ API Key للذكاء الاصطناعي في الإعدادات")
	}

	provider := strings.ToLower(strings.TrimSpace(settings.Provider))
	endpoint := strings.TrimSpace(settings.BaseURL)

	if endpoint == "" {
		switch provider {
		case "deepseek":
			endpoint = "https://api.deepseek.com"
		case "gemini":
			endpoint = "https://generativelanguage.googleapis.com/v1beta/openai"
		case "openai":
			endpoint = "https://api.openai.com/v1"
		default:
			endpoint = "https://api.deepseek.com"
		}
	}

	endpoint = strings.TrimSuffix(endpoint, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}

	if req.Model == "" {
		req.Model = settings.Model
	}
	if req.Model == "" {
		switch provider {
		case "deepseek":
			req.Model = "deepseek-chat"
		case "gemini":
			req.Model = "gemini-2.0-flash"
		case "openai":
			req.Model = "gpt-4o"
		default:
			req.Model = "deepseek-chat"
		}
	}

	if req.Temperature == 0 {
		req.Temperature = settings.Temperature
		if req.Temperature == 0 {
			req.Temperature = 0.2
		}
	}

	// Attempt request with automatic retry and model fallback on 503/429 errors
	var respBytes []byte
	var lastErr error

	modelsToTry := []string{req.Model}
	if provider == "gemini" {
		if req.Model == "gemini-2.0-flash" {
			modelsToTry = append(modelsToTry, "gemini-1.5-flash", "gemini-2.5-flash")
		} else if req.Model == "gemini-1.5-flash" {
			modelsToTry = append(modelsToTry, "gemini-2.0-flash")
		}
	}

	for _, modelName := range modelsToTry {
		req.Model = modelName
		bodyBytes, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("marshal request error: %w", err)
		}

		for attempt := 0; attempt < 2; attempt++ {
			if attempt > 0 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(1200 * time.Millisecond):
				}
			}

			httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
			if err != nil {
				return nil, fmt.Errorf("create request error: %w", err)
			}

			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)

			resp, err := c.httpClient.Do(httpReq)
			if err != nil {
				lastErr = fmt.Errorf("AI service request failed: %w", err)
				continue
			}

			respBytes, err = io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				lastErr = fmt.Errorf("read response error: %w", err)
				continue
			}

			if resp.StatusCode == 503 || resp.StatusCode == 429 {
				lastErr = fmt.Errorf("AI provider error (HTTP %d): %s", resp.StatusCode, string(respBytes))
				continue // retry or fallback model
			}

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return nil, fmt.Errorf("AI provider error (HTTP %d): %s", resp.StatusCode, string(respBytes))
			}

			var chatResp ChatCompletionResponse
			if err := json.Unmarshal(respBytes, &chatResp); err != nil {
				return nil, fmt.Errorf("unmarshal response error: %w (%s)", err, string(respBytes))
			}

			if chatResp.Error != nil {
				return nil, fmt.Errorf("AI error: %s", chatResp.Error.Message)
			}

			return &chatResp, nil
		}
	}

	return nil, lastErr
}
