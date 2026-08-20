package project

import (
	"context"
	"encoding/json"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
)

// TestSnapshotIncludesPreviewMesh проверяет, что экспортный документ
// содержит preview mesh (vertices + triangles) для 3D-визуализации.
func TestSnapshotIncludesPreviewMesh(t *testing.T) {
	res, err := stair.NewService().Calculate(context.Background(), testConfig(), stair.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Mesh == nil {
		t.Fatal("pipeline result must contain preview mesh")
	}

	snap := NewSnapshot("p-1", res)
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	mesh, ok := doc["mesh"].(map[string]any)
	if !ok {
		t.Fatal(`expected "mesh" key in snapshot`)
	}
	vertices, ok := mesh["Vertices"].([]any)
	if !ok || len(vertices) == 0 {
		t.Fatal(`expected non-empty "mesh.Vertices"`)
	}
	triangles, ok := mesh["Triangles"].([]any)
	if !ok || len(triangles) == 0 {
		t.Fatal(`expected non-empty "mesh.Triangles"`)
	}
}

// TestSnapshotOmitsMeshWhenAbsent — при blocking-валидации конвейер
// останавливается и mesh отсутствует (omitempty).
func TestSnapshotOmitsMeshWhenAbsent(t *testing.T) {
	res, err := stair.NewService().Calculate(context.Background(), testConfig(), stair.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res.Mesh = nil

	snap := NewSnapshot("p-1", res)
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if containsKey(t, raw, "mesh") {
		t.Fatal(`"mesh" must be omitted when nil`)
	}
}

func containsKey(t *testing.T, raw []byte, key string) bool {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	_, ok := doc[key]
	return ok
}

// TestSnapshotRailingEcho — снапшот несёт стороны перил (CONF-RAILING) для
// 2D-рендера в админке.
func TestSnapshotRailingEcho(t *testing.T) {
	res, err := stair.NewService().Calculate(context.Background(), testConfig(), stair.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res.Railing = engineering.RailingNone
	res.RailingLower = engineering.RailingRight

	snap := NewSnapshot("p-1", res)
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if v, ok := doc["railing"]; !ok || v != "none" {
		t.Fatalf(`expected railing: "none", got %v`, doc["railing"])
	}
	if v, ok := doc["railing_lower"]; !ok || v != "right" {
		t.Fatalf(`expected railing_lower: "right", got %v`, doc["railing_lower"])
	}
}
