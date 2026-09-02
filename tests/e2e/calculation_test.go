package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestStairCalculation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	client := NewClient("http://localhost:8080")
	err := WaitForServer(client.BaseURL, 2*time.Second)
	if err != nil {
		t.Skip("server not available, skipping E2E test")
	}

	// Регистрируемся
	uniqueEmail := fmt.Sprintf("calc-test-%d@example.com", time.Now().UnixNano())
	registerInput := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "TestPass123",
		"name":     "Calc Test User",
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

	// Калькулируем лестницу (используем :calculate вместо /calculate)
	calcInput := map[string]interface{}{
		"width_mm":  900,
		"height_mm": 2700,
		"flight":    "straight",
	}

	resp2, err := client.Post("/api/v1/stairs:calculate", calcInput)
	if err != nil {
		t.Fatalf("calculate stair request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		body, _ := ReadBody(resp2)
		t.Fatalf("calculate stair failed with status %d: %s", resp2.StatusCode, string(body))
	}

	var calcResp struct {
		Flight struct {
			StepCount    int     `json:"step_count"`
			AngleDeg     float64 `json:"angle_deg"`
			StepHeightMm float64 `json:"step_height_mm"`
			TreadDepthMm float64 `json:"tread_depth_mm"`
		} `json:"flight"`
		Validation struct {
			Valid bool `json:"valid"`
		} `json:"validation"`
	}

	if err := json.NewDecoder(resp2.Body).Decode(&calcResp); err != nil {
		t.Fatalf("failed to decode calculation response: %v", err)
	}

	if calcResp.Flight.StepCount == 0 {
		t.Error("expected step count > 0")
	}
	if calcResp.Flight.AngleDeg == 0 {
		t.Error("expected angle > 0")
	}
	if !calcResp.Validation.Valid {
		t.Error("expected valid calculation")
	}
}

func TestStairCalculationWithValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	client := NewClient("http://localhost:8080")
	err := WaitForServer(client.BaseURL, 2*time.Second)
	if err != nil {
		t.Skip("server not available, skipping E2E test")
	}

	// Регистрируемся
	uniqueEmail := fmt.Sprintf("validation-test-%d@example.com", time.Now().UnixNano())
	registerInput := map[string]interface{}{
		"email":    uniqueEmail,
		"password": "TestPass123",
		"name":     "Validation Test User",
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

	// Калькулируем лестницу с валидацией
	calcInput := map[string]interface{}{
		"width_mm":  1200,
		"height_mm": 3000,
		"flight":    "l_shape",
	}

	resp2, err := client.Post("/api/v1/stairs:calculate", calcInput)
	if err != nil {
		t.Fatalf("calculate stair request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		body, _ := ReadBody(resp2)
		t.Fatalf("calculate stair failed with status %d: %s", resp2.StatusCode, string(body))
	}

	var calcResp struct {
		Flight struct {
			StepCount int `json:"step_count"`
		} `json:"flight"`
		Validation struct {
			Valid  bool `json:"valid"`
			Issues []struct {
				Code     string `json:"code"`
				Severity string `json:"severity"`
				Message  string `json:"message"`
			} `json:"issues"`
		} `json:"validation"`
	}

	if err := json.NewDecoder(resp2.Body).Decode(&calcResp); err != nil {
		t.Fatalf("failed to decode calculation response: %v", err)
	}

	// L-shape должна иметь step count > 0
	if calcResp.Flight.StepCount == 0 {
		t.Error("expected step count > 0")
	}
}
