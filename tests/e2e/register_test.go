package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestUserRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	client := NewClient("http://localhost:8080")
	err := WaitForServer(client.BaseURL, 2*time.Second)
	if err != nil {
		t.Skip("server not available, skipping E2E test")
	}

	// Уникальный email для каждого запуска
	uniqueEmail := fmt.Sprintf("e2e-test-%d@example.com", time.Now().UnixNano())

	// Регистрируем нового пользователя
	registerInput := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "TestPass123",
		"name":     "E2E Test User",
	}

	resp, err := client.Post("/api/v1/auth/register", registerInput)
	if err != nil {
		t.Fatalf("registration request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, _ := ReadBody(resp)
		t.Fatalf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	var registerResp struct {
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
		Token string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&registerResp); err != nil {
		t.Fatalf("failed to decode registration response: %v", err)
	}

	if registerResp.User.ID == "" {
		t.Error("expected user ID")
	}
	if registerResp.User.Email == "" {
		t.Error("expected user email")
	}
	if registerResp.Token == "" {
		t.Error("expected token")
	}

	// Проверяем что можем получить профиль
	resp2, err := client.Get("/api/v1/auth/me")
	if err != nil {
		t.Fatalf("get profile request failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusOK {
		body, _ := ReadBody(resp2)
		t.Fatalf("get profile failed with status %d: %s", resp2.StatusCode, string(body))
	}
}

func TestUserLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	client := NewClient("http://localhost:8080")
	err := WaitForServer(client.BaseURL, 2*time.Second)
	if err != nil {
		t.Skip("server not available, skipping E2E test")
	}

	// Уникальный email для каждого запуска
	uniqueEmail := fmt.Sprintf("e2e-test-%d@example.com", time.Now().UnixNano())

	// Сначала регистрируем пользователя
	registerInput := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "TestPass123",
		"name":     "E2E Login Test User",
	}

	resp, err := client.Post("/api/v1/auth/register", registerInput)
	if err != nil {
		t.Fatalf("registration request failed: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := ReadBody(resp)
		t.Fatalf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Логинимся
	loginInput := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "TestPass123",
	}

	resp2, err := client.Post("/api/v1/auth/login", loginInput)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusOK {
		body, _ := ReadBody(resp2)
		t.Fatalf("login failed with status %d: %s", resp2.StatusCode, string(body))
	}

	var loginResp struct {
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
		Token string `json:"token"`
	}

	if err := json.NewDecoder(resp2.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	if loginResp.Token == "" {
		t.Error("expected token")
	}
}
