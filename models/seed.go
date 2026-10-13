// Package models — сид-манифест (S-203).
//
// models/seed/seed-manifest.json генерируется тем же конвейером, что и
// эталонный (hack/gen_part_manifest.py через STAIR_PARTS_YAML), из
// авторской модели models/seed/minimal-straight.FCStd.
// Вшит через go:embed — те же причины, что у parts-manifest.json.
package models

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed seed/seed-manifest.json
var seedManifestJSON []byte

// SeedLoad разбирает вшитый сид-манифест. Пустой (0 деталей) — ошибка:
// молчаливый пустой манифест приняли бы за «перил нет».
func SeedLoad() (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(seedManifestJSON, &m); err != nil {
		return Manifest{}, fmt.Errorf("models: разбор seed-manifest.json: %w", err)
	}
	if len(m.Parts) == 0 {
		return Manifest{}, fmt.Errorf("models: seed-manifest.json без деталей")
	}
	return m, nil
}
