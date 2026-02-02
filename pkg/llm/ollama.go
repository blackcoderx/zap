// Package llm provides client implementations for Large Language Models,
// specifically focusing on Ollama integration for local AI inference.
package llm

import (
	"bufio"
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

// StreamCallback is called for each chunk of streaming response
type StreamCallback func(chunk string)

// OllamaClient handles communication with Ollama API
type OllamaClient struct {
	BaseURL         string
	Model           string
	APIKey          string
	HTTPClient      *http.Client // Client with timeout for regular requests
	StreamingClient *http.Client // Client without timeout for streaming
}

// NewOllamaClient creates a new Ollama client with proper connection pooling.
// Two HTTP clients are created:
//   - HTTPClient: For regular requests with a 60-second timeout
//   - StreamingClient: For streaming requests without timeout (connections can be long-lived)
func NewOllamaClient(baseURL, model, apiKey string) *OllamaClient {
	return &OllamaClient{
		BaseURL: baseURL,
		Model:   model,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		StreamingClient: &http.Client{
			Timeout: 0, // No timeout for streaming - responses can take a while
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

// ChatStream sends a chat request with streaming and calls callback for each chunk.
// If streaming fails with 503 (common with Ollama Cloud), it automatically falls back
// to non-streaming mode and delivers the response as a single chunk.
func (c *OllamaClient) ChatStream(messages []Message, callback StreamCallback) (string, error) {
	req := ChatRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   true,
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

	// Use the dedicated streaming client (no timeout, connection reuse)
	resp, err := c.StreamingClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// If streaming returns 503 (common with Ollama Cloud), fall back to non-streaming
	if resp.StatusCode == http.StatusServiceUnavailable {
		resp.Body.Close() // Close the failed streaming response
		return c.chatWithFallback(messages, callback)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read streaming response line by line
	var fullContent string
	var malformedLines int
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var chatResp ChatResponse
		if err := json.Unmarshal([]byte(line), &chatResp); err != nil {
			// Track malformed lines for debugging but don't fail
			// This can happen with non-JSON status messages from some servers
			malformedLines++
			continue
		}

		chunk := chatResp.Message.Content
		if chunk != "" {
			fullContent += chunk
			if callback != nil {
				callback(chunk)
			}
		}

		if chatResp.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		// Include malformed line count in error for debugging
		if malformedLines > 0 {
			return fullContent, fmt.Errorf("error reading stream (skipped %d malformed lines): %w", malformedLines, err)
		}
		return fullContent, fmt.Errorf("error reading stream: %w", err)
	}

	return fullContent, nil
}

// chatWithFallback uses non-streaming mode and delivers the response via callback.
// This is used as a fallback when streaming is unavailable (e.g., Ollama Cloud 503).
func (c *OllamaClient) chatWithFallback(messages []Message, callback StreamCallback) (string, error) {
	content, err := c.Chat(messages)
	if err != nil {
		return "", err
	}

	// Deliver the full response as a single "chunk" via the callback
	if callback != nil && content != "" {
		callback(content)
	}

	return content, nil
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
