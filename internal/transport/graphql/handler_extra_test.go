package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
	domevents "stairplatform/internal/domain/events"
)

func postGraphQL(t *testing.T, h *HTTPHandler, req Request) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	return rr
}

func decodeResponse(t *testing.T, rr *httptest.ResponseRecorder) Response {
	t.Helper()
	var resp Response
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp
}

func TestHandlerResolverGetter(t *testing.T) {
	r := newTestResolver()
	h := NewHTTPHandler(r)
	if h.Resolver() != r {
		t.Fatal("Resolver() must return the injected resolver")
	}
}

func TestHTTPHandlerCreateStairConfiguration(t *testing.T) {
	h := NewHTTPHandler(newTestResolver())
	rr := postGraphQL(t, h, Request{
		Query: "mutation { createStairConfiguration(input: {...}) { id } }",
		Variables: map[string]interface{}{
			"input": map[string]interface{}{
				"projectId":  "p-1",
				"name":       "Main stairs",
				"width":      950.0,
				"height":     2700.0,
				"flightType": "straight",
			},
		},
	})
	resp := decodeResponse(t, rr)
	if len(resp.Errors) != 0 {
		t.Fatalf("unexpected errors: %+v", resp.Errors)
	}
	if resp.Data == nil {
		t.Fatal("expected data")
	}
}

func TestHTTPHandlerStairConfigurationNotFound(t *testing.T) {
	h := NewHTTPHandler(newTestResolver())
	rr := postGraphQL(t, h, Request{
		Query:     "query { stairConfiguration(id: \"ghost\") { id } }",
		Variables: map[string]interface{}{"id": "ghost"},
	})
	resp := decodeResponse(t, rr)
	if len(resp.Errors) == 0 {
		t.Fatal("expected error for unknown configuration")
	}
}

func TestHTTPHandlerProjectConfigurations(t *testing.T) {
	r := newTestResolver()
	if _, err := r.Mutation().CreateStairConfiguration(context.Background(), CreateStairInput{Width: 900, Height: 2700, FlightType: "straight"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	h := NewHTTPHandler(r)
	rr := postGraphQL(t, h, Request{
		Query:     "query { projectConfigurations(projectId: \"default\") { id } }",
		Variables: map[string]interface{}{"projectId": "default"},
	})
	resp := decodeResponse(t, rr)
	if len(resp.Errors) != 0 || resp.Data == nil {
		t.Fatalf("ProjectConfigurations: errors=%+v data=%v", resp.Errors, resp.Data)
	}
}

func TestHTTPHandlerRunAnalysis(t *testing.T) {
	r := newTestResolver()
	created, err := r.Mutation().CreateStairConfiguration(context.Background(), CreateStairInput{Width: 900, Height: 2700, FlightType: "straight"})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	h := NewHTTPHandler(r)
	rr := postGraphQL(t, h, Request{
		Query:     "mutation { runAnalysis(configId: \"x\") { status } }",
		Variables: map[string]interface{}{"configId": created.ID},
	})
	if resp := decodeResponse(t, rr); len(resp.Errors) != 0 {
		t.Fatalf("RunAnalysis: unexpected errors: %+v", resp.Errors)
	}

	// Неизвестная конфигурация → ошибка.
	rr2 := postGraphQL(t, h, Request{
		Query:     "mutation { runAnalysis(configId: \"ghost\") { status } }",
		Variables: map[string]interface{}{"configId": "ghost"},
	})
	if resp2 := decodeResponse(t, rr2); len(resp2.Errors) == 0 {
		t.Fatal("expected error for unknown config")
	}
}

func TestHTTPHandlerRunPipeline(t *testing.T) {
	r := newTestResolver()
	created, err := r.Mutation().CreateStairConfiguration(context.Background(), CreateStairInput{Width: 900, Height: 2700, FlightType: "straight"})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	h := NewHTTPHandler(r)
	rr := postGraphQL(t, h, Request{
		Query:     "mutation { runPipeline(configId: \"x\") { status } }",
		Variables: map[string]interface{}{"configId": created.ID},
	})
	if resp := decodeResponse(t, rr); len(resp.Errors) != 0 {
		t.Fatalf("RunPipeline: unexpected errors: %+v", resp.Errors)
	}

	rr2 := postGraphQL(t, h, Request{
		Query:     "mutation { runPipeline(configId: \"ghost\") { status } }",
		Variables: map[string]interface{}{"configId": "ghost"},
	})
	if resp2 := decodeResponse(t, rr2); len(resp2.Errors) == 0 {
		t.Fatal("expected error for unknown config")
	}
}

func TestQueryProjectConfigurationsAndAnalysis(t *testing.T) {
	r := newTestResolver()
	ctx := context.Background()

	r.configs["c1"] = &stair.Config{
		Width:  engineering.Length(900),
		Height: engineering.Length(2700),
		Flight: engineering.FlightType("straight"),
	}
	r.configs["c2"] = nil

	dtos, err := r.Query().ProjectConfigurations(ctx, "default")
	if err != nil {
		t.Fatalf("ProjectConfigurations: %v", err)
	}
	if len(dtos) != 1 {
		t.Fatalf("want 1 dto (nil config skipped), got %d", len(dtos))
	}

	analysis, err := r.Query().Analysis(ctx, "any")
	if err != nil {
		t.Fatalf("Analysis: %v", err)
	}
	if analysis.Status != "completed" || analysis.ID != "any" {
		t.Fatalf("Analysis = %+v", analysis)
	}
}

func TestCreateStairConfigurationOptionals(t *testing.T) {
	r := newTestResolver()
	ctx := context.Background()

	sh, st, th := 170.0, 60.0, 30.0
	dto, err := r.Mutation().CreateStairConfiguration(ctx, CreateStairInput{
		ProjectID: "p-1", Name: "N", Width: 900, Height: 2700, FlightType: "straight",
		StepHeight: &sh, StringerThickness: &st, StepThickness: &th,
	})
	if err != nil {
		t.Fatalf("CreateStairConfiguration: %v", err)
	}
	if dto.StepHeight != 170 || dto.StringerThickness != 60 || dto.StepThickness != 30 {
		t.Fatalf("optionals not applied: %+v", dto)
	}

	dto2, err := r.Mutation().CreateStairConfiguration(ctx, CreateStairInput{Width: 900, Height: 2700, FlightType: "straight"})
	if err != nil {
		t.Fatalf("CreateStairConfiguration default: %v", err)
	}
	if dto2.StringerThickness != 50 || dto2.StepThickness != 40 {
		t.Fatalf("defaults not applied: %+v", dto2)
	}
}

func TestResolverErrorPaths(t *testing.T) {
	r := newTestResolver()
	ctx := context.Background()

	if _, err := r.Mutation().UpdateStairConfiguration(ctx, "ghost", UpdateStairInput{Name: new(string)}); err == nil {
		t.Fatal("UpdateStairConfiguration(ghost): want error")
	}
	if _, err := r.Mutation().RunAnalysis(ctx, "ghost"); err == nil {
		t.Fatal("RunAnalysis(ghost): want error")
	}
	if _, err := r.Mutation().RunPipeline(ctx, "ghost"); err == nil {
		t.Fatal("RunPipeline(ghost): want error")
	}
	if _, err := r.Query().StairConfiguration(ctx, "ghost"); err == nil {
		t.Fatal("StairConfiguration(ghost): want error")
	}
}

func TestSearchStairsLimit(t *testing.T) {
	r := newTestResolver()
	ctx := context.Background()
	r.configs["a"] = &stair.Config{
		Width:  engineering.Length(900),
		Height: engineering.Length(2700),
		Flight: engineering.FlightType("straight"),
	}
	r.configs["b"] = &stair.Config{
		Width:  engineering.Length(1000),
		Height: engineering.Length(2800),
		Flight: engineering.FlightType("straight"),
	}

	lim := 1
	got, err := r.Query().SearchStairs(ctx, SearchParams{Limit: &lim})
	if err != nil {
		t.Fatalf("SearchStairs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 result with limit, got %d", len(got))
	}
}

func TestSubscriptionsDeliverEvents(t *testing.T) {
	r := newTestResolver()
	ctx := context.Background()

	statusCh, err := r.Subscription().PipelineStatusChanged(ctx, "cfg-1")
	if err != nil {
		t.Fatalf("PipelineStatusChanged: %v", err)
	}
	progressCh, err := r.Subscription().AnalysisProgress(ctx, "cfg-1")
	if err != nil {
		t.Fatalf("AnalysisProgress: %v", err)
	}
	notifCh, err := r.Subscription().Notifications(ctx, "u-1")
	if err != nil {
		t.Fatalf("Notifications: %v", err)
	}

	// Publish исполняет обработчики синхронно → канал заполнен до чтения.
	if err := r.bus.Publish(ctx, domevents.PipelineCompleted{
		BaseEvent: domevents.BaseEvent{Meta: domevents.NewEventMetadata(domevents.EventPipelineCompleted, "test", "system", "t1", "", "")},
		ConfigID:  "cfg-1",
	}); err != nil {
		t.Fatalf("publish pipeline: %v", err)
	}
	if err := r.bus.Publish(ctx, domevents.AnalysisCompleted{
		BaseEvent: domevents.BaseEvent{Meta: domevents.NewEventMetadata(domevents.EventAnalysisCompleted, "test", "system", "t1", "", "")},
		ConfigID:  "cfg-1",
	}); err != nil {
		t.Fatalf("publish analysis: %v", err)
	}
	if err := r.bus.Publish(ctx, domevents.AnalysisStarted{
		BaseEvent: domevents.BaseEvent{Meta: domevents.NewEventMetadata(domevents.EventAnalysisStarted, "test", "system", "t1", "", "")},
		ConfigID:  "cfg-1",
	}); err != nil {
		t.Fatalf("publish started: %v", err)
	}

	if dto := <-statusCh; dto.Status != "completed" || dto.ConfigID != "cfg-1" {
		t.Fatalf("status dto = %+v", dto)
	}
	if dto := <-progressCh; dto.Status != "completed" {
		t.Fatalf("progress dto = %+v", dto)
	}
	if n := <-notifCh; n.ID == "" || n.Type == "" || n.Message == "" {
		t.Fatalf("notification = %+v", n)
	}
}
