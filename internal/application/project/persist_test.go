package project

import (
	"encoding/json"
	"testing"

	"stairplatform/internal/application/stair"
)

// TestSnapshotIncludesPreviewMesh проверяет, что экспортный документ
// содержит preview mesh (vertices + triangles) для 3D-визуализации.
func TestSnapshotIncludesPreviewMesh(t *testing.T) {
	res, err := stair.NewService().Calculate(testConfig(), stair.Options{})
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
	res, err := stair.NewService().Calculate(testConfig(), stair.Options{})
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
