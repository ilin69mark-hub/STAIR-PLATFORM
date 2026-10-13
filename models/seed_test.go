package models

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Гейты сид-манифеста (S-203): файл разбирается, 16 позиций, объёмы
// физически возможны, sha256 модели совпадает, экземпляры не превышают
// число тел (instances ≤ count: у составного фитинга тел больше,
// чем изделий, — наоборот быть не может).

func seedOrFail(t *testing.T) Manifest {
	t.Helper()
	m, err := SeedLoad()
	if err != nil {
		t.Fatalf("вшитый сид-манифест не разобрался: %v", err)
	}
	if len(m.Parts) != 16 {
		t.Fatalf("в сид-манифесте %d деталей, ожидалось 16", len(m.Parts))
	}
	return m
}

func TestSeedManifestSane(t *testing.T) {
	m := seedOrFail(t)
	for _, p := range m.Parts {
		if p.Volume <= 0 {
			t.Errorf("%s: неположительный объём", p.ID)
		}
		if bv := p.BBox.L * p.BBox.W * p.BBox.H; p.Volume > bv {
			t.Errorf("%s: объём %.1f больше габарита %.1f", p.ID, p.Volume, bv)
		}
		if p.Instances < 1 {
			t.Errorf("%s: нет числа изделий", p.ID)
		}
		if p.Instances > len(p.Solids) {
			t.Errorf("%s: изделий %d больше тел %d", p.ID, p.Instances, len(p.Solids))
		}
	}
}

func TestSeedManifestModelFresh(t *testing.T) {
	m := seedOrFail(t)
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller не отдал путь теста")
	}
	// models → корень репозитория.
	repo := filepath.Join(filepath.Dir(thisFile), "..")
	modelPath := filepath.Join(repo, "models", m.SourceModel)
	//nolint:gosec // G304: путь из вшитого манифеста и положения теста.
	buf, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("модель %s не читается: %v", modelPath, err)
	}
	sum := sha256.Sum256(buf)
	if got := hex.EncodeToString(sum[:]); got != m.SourceSHA256 {
		t.Errorf("модель изменилась без регенерации сид-манифеста:\n файл: %s\n манифест: %s",
			got, m.SourceSHA256)
	}
}
