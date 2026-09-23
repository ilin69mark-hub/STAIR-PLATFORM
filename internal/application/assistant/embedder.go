package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// EmbeddingDim — размерность эмбеддингов корпуса (schema: vector(1536),
// S-135). Модели эмбеддеров OpenAI-совместимых API с другой размерностью
// отклоняются на этапе backfill — схема БД фиксирует 1536.
const EmbeddingDim = 1536

// Embedder — порт вычисления эмбеддингов текста (S-135, AI-0007).
// Реализуется OpenAI-совместимым /embeddings клиентом; в тестах — фейками.
type Embedder interface {
	// Embed возвращает по одному вектору размерности EmbeddingDim на текст
	// (порядок сохраняется).
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// OpenAIEmbedder — эмбеддер OpenAI-совместимого API (OpenAI, OpenRouter,
// Ollama, vLLM) через POST {baseURL}/embeddings. Чистая stdlib (паттерн
// openai.go, SSO G5). Подключается только при конфигурации STAIR_AI_EMBED_*.
type OpenAIEmbedder struct {
	client      *http.Client
	baseURL     string
	apiKey      string
	model       string
	timeout     time.Duration
	maxResponse int64
	headers     map[string]string // доп. заголовки (OpenRouter: HTTP-Referer/X-Title)
}

// EmbedderConfig — параметры эмбеддера.
type EmbedderConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	// Timeout — таймаут одного запроса; 0 → 15s.
	Timeout time.Duration
	// MaxResponse — предельный размер ответа; 0 → 16 MiB.
	MaxResponse int64
	// Client — HTTP-клиент; nil → http.DefaultClient.
	Client *http.Client
	// Headers — дополнительные заголовки каждого запроса
	// (например HTTP-Referer/X-Title для OpenRouter).
	Headers map[string]string
}

// NewOpenAIEmbedder создаёт эмбеддер; BaseURL обязателен.
func NewOpenAIEmbedder(c EmbedderConfig) (*OpenAIEmbedder, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return nil, fmt.Errorf("assistant: embedder base URL required")
	}
	if strings.TrimSpace(c.Model) == "" {
		return nil, fmt.Errorf("assistant: embedder model required")
	}
	if c.Timeout <= 0 {
		c.Timeout = 15 * time.Second
	}
	if c.MaxResponse <= 0 {
		c.MaxResponse = 16 << 20
	}
	if c.Client == nil {
		c.Client = http.DefaultClient
	}
	headers := make(map[string]string, len(c.Headers))
	for k, v := range c.Headers {
		if k != "" && v != "" {
			headers[k] = v
		}
	}
	return &OpenAIEmbedder{
		client:      c.Client,
		baseURL:     strings.TrimRight(c.BaseURL, "/"),
		apiKey:      c.APIKey,
		model:       c.Model,
		timeout:     c.Timeout,
		maxResponse: c.MaxResponse,
		headers:     headers,
	}, nil
}

// embedRequest — тело запроса /embeddings.
type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embedData — элемент ответа /embeddings.
type embedData struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// embedResponse — ответ /embeddings.
type embedResponse struct {
	Data []embedData `json:"data"`
}

// Embed реализует Embedder: батчем до batchSize (64), размерность строго
// EmbeddingDim. Ошибка валидации размерности — фатальна для backfill.
func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	const batchSize = 64
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += batchSize {
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		res, err := e.embedBatch(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, res...)
	}
	return out, nil
}

func (e *OpenAIEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(embedRequest{Model: e.model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("assistant: embedder marshal: %w", err)
	}

	cctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(cctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("assistant: embedder request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("assistant: embedder call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("assistant: embedder status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, e.maxResponse))
	if err != nil {
		return nil, fmt.Errorf("assistant: embedder read: %w", err)
	}
	var er embedResponse
	if err := json.Unmarshal(raw, &er); err != nil {
		return nil, fmt.Errorf("assistant: embedder decode: %w", err)
	}
	if len(er.Data) != len(texts) {
		return nil, fmt.Errorf("assistant: embedder returned %d vectors for %d texts", len(er.Data), len(texts))
	}
	// Гарантия порядка: сортировка по index (API может вернуть вразнобой).
	sorted := append([]embedData(nil), er.Data...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Index < sorted[j].Index })
	out := make([][]float32, 0, len(sorted))
	for _, d := range sorted {
		if len(d.Embedding) != EmbeddingDim {
			return nil, fmt.Errorf("assistant: embedder returned %d-dim vector, want %d (model %q)", len(d.Embedding), EmbeddingDim, e.model)
		}
		out = append(out, d.Embedding)
	}
	return out, nil
}
