package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Message represents a chat message
type Message struct {
	Role    string `json:"role"` // "system", "user", or "assistant"
	Content string `json:"content"`
}

// ChatRequest represents an Ollama chat request
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// ChatResponse represents an Ollama chat response
type ChatResponse struct {
	Model     string  `json:"model"`
	CreatedAt string  `json:"created_at"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
}

// OllamaClient handles communication with Ollama API
type OllamaClient struct {
	BaseURL    string
	Model      string
	APIKey     string
	HTTPClient *http.Client
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL, model, apiKey string) *OllamaClient {
	return &OllamaClient{
		BaseURL: baseURL,
		Model:   model,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Chat sends a chat request to Ollama and returns the response
func (c *OllamaClient) Chat(messages []Message) (string, error) {
	req := ChatRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   false,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/chat", c.BaseURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama (url: %s, model: %s) returned status %d: %s", url, c.Model, resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return chatResp.Message.Content, nil
}

// CheckConnection verifies that Ollama is running and accessible
func (c *OllamaClient) CheckConnection() error {
	url := fmt.Sprintf("%s/api/tags", c.BaseURL)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	return nil
}
