package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestDocumentExport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	client := NewClient("http://localhost:8080")
	err := WaitForServer(client.BaseURL, 2*time.Second)
	if err != nil {
		t.Skip("server not available, skipping E2E test")
	}

	// Регистрируемся
	uniqueEmail := fmt.Sprintf("export-test-%d@example.com", time.Now().UnixNano())
	registerInput := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "TestPass123",
		"name":     "Export Test User",
	}

	resp, err := client.Post("/api/v1/auth/register", registerInput)
	if err != nil {
		t.Fatalf("registration request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var registerResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&registerResp); err != nil {
		t.Fatal(err)
	}

	// Создаем проект
	createInput := map[string]interface{}{
		"name":        "Export Test Project",
		"description": "For export testing",
	}

	resp2, err := client.Post("/api/v1/projects", createInput)
	if err != nil {
		t.Fatalf("create project request failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	var projectResp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&projectResp); err != nil {
		t.Fatal(err)
	}

	if projectResp.ID == "" {
		t.Fatal("expected project ID")
	}

	// Экспортируем проект в JSON (GET /api/v1/projects/{id}/export)
	resp3, err := client.Get("/api/v1/projects/" + projectResp.ID + "/export")
	if err != nil {
		t.Fatalf("export request failed: %v", err)
	}
	defer func() { _ = resp3.Body.Close() }()

	// Ожидаем 200 или 404 (если нет сохраненной конфигурации)
	if resp3.StatusCode != http.StatusOK && resp3.StatusCode != http.StatusNotFound {
		body, _ := ReadBody(resp3)
		t.Fatalf("export failed with status %d: %s", resp3.StatusCode, string(body))
	}

	// Если есть данные — проверяем что это JSON
	if resp3.StatusCode == http.StatusOK {
		contentType := resp3.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected application/json content type, got %q", contentType)
		}
	}
}

func TestCNCExport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	client := NewClient("http://localhost:8080")
	err := WaitForServer(client.BaseURL, 2*time.Second)
	if err != nil {
		t.Skip("server not available, skipping E2E test")
	}

	// Регистрируемся
	uniqueEmail := fmt.Sprintf("cnc-test-%d@example.com", time.Now().UnixNano())
	registerInput := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "TestPass123",
		"name":     "CNC Test User",
	}

	resp, err := client.Post("/api/v1/auth/register", registerInput)
	if err != nil {
		t.Fatalf("registration request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var registerResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&registerResp); err != nil {
		t.Fatal(err)
	}

	// Создаем проект
	createInput := map[string]interface{}{
		"name":        "CNC Test Project",
		"description": "For CNC export testing",
	}

	resp2, err := client.Post("/api/v1/projects", createInput)
	if err != nil {
		t.Fatalf("create project request failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	var projectResp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&projectResp); err != nil {
		t.Fatal(err)
	}

	if projectResp.ID == "" {
		t.Fatal("expected project ID")
	}

	// Экспортируем в CAD (GET /api/v1/projects/{id}/export/cad)
	resp3, err := client.Get("/api/v1/projects/" + projectResp.ID + "/export/cad?format=stl")
	if err != nil {
		t.Fatalf("CNC export request failed: %v", err)
	}
	defer func() { _ = resp3.Body.Close() }()

	// Ожидаем 200 или 404 (если нет сохраненной конфигурации)
	if resp3.StatusCode != http.StatusOK && resp3.StatusCode != http.StatusNotFound {
		body, _ := ReadBody(resp3)
		t.Fatalf("CNC export failed with status %d: %s", resp3.StatusCode, string(body))
	}

	// Если есть данные — проверяем что это бинарный файл
	if resp3.StatusCode == http.StatusOK {
		contentType := resp3.Header.Get("Content-Type")
		if contentType == "" {
			t.Error("expected content type header")
		}
	}
}
