package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// Client — HTTP client for E2E tests.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
}

// NewClient создаёт новый E2E client.
func NewClient(baseURL string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
	}
}

// SetToken устанавливает JWT token.
func (c *Client) SetToken(token string) {
	c.Token = token
}

// Request выполняет HTTP запрос.
func (c *Client) Request(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Session-аутентификация: после register/login клиент хранит session
	// и csrf cookie (как фронтенд). Сессия идёт через cookie, а не Bearer:
	// requireAuth принимает Authorization: Bearer только как API-ключ (EDR-0016).
	// Для мутирующих запросов проставляем X-CSRF-Token (double-submit).
	if c.Token == "" {
		csrf := c.csrfToken(req)
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
	} else {
		// API-ключ: не-браузерный клиент, CSRF не требуется (EDR-0016 §7).
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	return c.HTTPClient.Do(req)
}

// csrfToken возвращает csrf-cookie из jar (double-submit), если она есть.
func (c *Client) csrfToken(req *http.Request) string {
	for _, cookie := range c.HTTPClient.Jar.Cookies(req.URL) {
		if cookie.Name == "csrf" {
			return cookie.Value
		}
	}
	return ""
}

// Post выполняет POST запрос.
func (c *Client) Post(path string, body interface{}) (*http.Response, error) {
	return c.Request(http.MethodPost, path, body)
}

// Get выполняет GET запрос.
func (c *Client) Get(path string) (*http.Response, error) {
	return c.Request(http.MethodGet, path, nil)
}

// Put выполняет PUT запрос.
func (c *Client) Put(path string, body interface{}) (*http.Response, error) {
	return c.Request(http.MethodPut, path, body)
}

// Patch выполняет PATCH запрос.
func (c *Client) Patch(path string, body interface{}) (*http.Response, error) {
	return c.Request(http.MethodPatch, path, body)
}

// Delete выполняет DELETE запрос.
func (c *Client) Delete(path string) (*http.Response, error) {
	return c.Request(http.MethodDelete, path, nil)
}

// ReadBody читает тело ответа.
func ReadBody(resp *http.Response) ([]byte, error) {
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}

// WaitForServer ждёт пока сервер будет доступен.
func WaitForServer(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/ready")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("server not ready after %v", timeout)
}
