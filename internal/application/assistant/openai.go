package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAI — первичный бэкенд, совместимый с OpenAI Chat Completions API
// (OpenAI/Anthropic-compatible gateway, Ollama, vLLM и т.п.). Реализован на
// чистой stdlib (паттерн SSO G5). Подключается только при наличии
// конфигурации STAIR_AI_* (AI-0017); по умолчанию используется локальный
// детерминированный бэкенд (AI-0003).
type OpenAI struct {
	client      *http.Client
	baseURL     string // например https://api.openai.com/v1 — к нему добавляется /chat/completions
	apiKey      string
	model       string
	timeout     time.Duration
	maxResponse int64
}

// OpenAIConfig — параметры OpenAI-совместимого бэкенда.
type OpenAIConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	// Timeout — таймаут одного запроса; 0 → 15s.
	Timeout time.Duration
	// MaxResponse — предельный размер ответа; 0 → 256 KiB.
	MaxResponse int64
	// Client — HTTP-клиент; nil → http.DefaultClient.
	Client *http.Client
}

// NewOpenAI создаёт бэкенд. Возвращает ошибку при пустом BaseURL.
func NewOpenAI(c OpenAIConfig) (*OpenAI, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return nil, fmt.Errorf("assistant: OpenAI base URL required")
	}
	if c.Timeout <= 0 {
		c.Timeout = 15 * time.Second
	}
	if c.MaxResponse <= 0 {
		c.MaxResponse = 256 << 10
	}
	if c.Client == nil {
		c.Client = http.DefaultClient
	}
	return &OpenAI{
		client:      c.Client,
		baseURL:     strings.TrimRight(c.BaseURL, "/"),
		apiKey:      c.APIKey,
		model:       c.Model,
		timeout:     c.Timeout,
		maxResponse: c.MaxResponse,
	}, nil
}

// chatMessage — сообщение в формате Chat Completions API.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest — тело запроса Chat Completions API.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

// chatResponse — ответ Chat Completions API (интересует только content).
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Infer реализует Backend: системная инструкция + пользовательский буфер
// (задача + контекст тулов). Только структурные данные контекста: никаких
// персональных и аудируемых данных (AI-0001, AI-0012).
func (o *OpenAI) Infer(ctx context.Context, p Prompt) (*Answer, error) {
	if o.model == "" {
		return nil, fmt.Errorf("assistant: openai model not configured")
	}
	system := p.Instructions
	if system == "" {
		system = "Ты — эксперт по лестничным конструкциям. Отвечай по-русски, кратко и по существу."
	}
	user := "Задача: " + p.UserTask + "\n\nКонтекст результатов:\n" + p.Context
	if len(p.StructuredData) > 0 {
		user += "\n\nСтруктурированные данные:\n" + string(p.StructuredData)
	}

	body, err := json.Marshal(chatRequest{
		Model: o.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0, // детерминизм комментария (ADR-0003, по возможности)
	})
	if err != nil {
		return nil, fmt.Errorf("assistant: openai marshal: %w", err)
	}

	cctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(cctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("assistant: openai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("assistant: openai call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("assistant: openai status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, o.maxResponse))
	if err != nil {
		return nil, fmt.Errorf("assistant: openai read: %w", err)
	}
	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return nil, fmt.Errorf("assistant: openai decode: %w", err)
	}
	if len(cr.Choices) == 0 {
		return &Answer{}, nil // пустой ответ — ModelRouter сделает фолбэк на local
	}
	return &Answer{Text: cr.Choices[0].Message.Content}, nil
}
