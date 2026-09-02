package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestProjectCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	client := NewClient("http://localhost:8080")
	err := WaitForServer(client.BaseURL, 2*time.Second)
	if err != nil {
		t.Skip("server not available, skipping E2E test")
	}

	// Регистрируемся
	uniqueEmail := fmt.Sprintf("project-test-%d@example.com", time.Now().UnixNano())
	registerInput := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "TestPass123",
		"name":     "Project Test User",
	}

	resp, err := client.Post("/api/v1/auth/register", registerInput)
	if err != nil {
		t.Fatalf("registration request failed: %v", err)
	}
	defer resp.Body.Close()

	var registerResp struct {
		Token string `json:"token"`
	}
	json.NewDecoder(resp.Body).Decode(&registerResp)
	client.SetToken(registerResp.Token)

	// Создаем проект
	createInput := map[string]interface{}{
		"name":        "Test Project",
		"description": "E2E test project",
	}

	resp2, err := client.Post("/api/v1/projects", createInput)
	if err != nil {
		t.Fatalf("create project request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusCreated {
		body, _ := ReadBody(resp2)
		t.Fatalf("create project failed with status %d: %s", resp2.StatusCode, string(body))
	}

	var projectResp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	json.NewDecoder(resp2.Body).Decode(&projectResp)

	if projectResp.ID == "" {
		t.Error("expected project ID")
	}
	if projectResp.Name != "Test Project" {
		t.Errorf("expected project name 'Test Project', got %q", projectResp.Name)
	}

	// Получаем проект
	resp3, err := client.Get("/api/v1/projects/" + projectResp.ID)
	if err != nil {
		t.Fatalf("get project request failed: %v", err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusOK {
		body, _ := ReadBody(resp3)
		t.Fatalf("get project failed with status %d: %s", resp3.StatusCode, string(body))
	}

	// Обновляем проект
	updateInput := map[string]interface{}{
		"name": "Updated Project",
	}

	resp4, err := client.Put("/api/v1/projects/"+projectResp.ID, updateInput)
	if err != nil {
		t.Fatalf("update project request failed: %v", err)
	}
	defer resp4.Body.Close()

	if resp4.StatusCode != http.StatusOK {
		body, _ := ReadBody(resp4)
		t.Fatalf("update project failed with status %d: %s", resp4.StatusCode, string(body))
	}

	// Удаляем проект
	resp5, err := client.Delete("/api/v1/projects/" + projectResp.ID)
	if err != nil {
		t.Fatalf("delete project request failed: %v", err)
	}
	defer resp5.Body.Close()

	if resp5.StatusCode != http.StatusNoContent {
		body, _ := ReadBody(resp5)
		t.Fatalf("delete project failed with status %d: %s", resp5.StatusCode, string(body))
	}
}
