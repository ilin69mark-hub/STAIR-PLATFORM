package assistant

// Бенчмарк сквозного Ask+RAG+memory (S-135): детерминированный путь
// (локальный бэкенд + фейковые ретривер/память с 5 чанками) — нижняя граница
// латентности конвейера без задержек эмбеддера/LLM (DoD S-135: p95 Ask+RAG < 5с;
// внешние вызовы покрыты таймаутами: эмбеддер 15s, primary backend — см. openai.go).

import (
	"context"
	"testing"

	"stairplatform/internal/application/stair"
	"stairplatform/internal/domain/engineering"
)

func BenchmarkAskWithRAG(b *testing.B) {
	calc := &fakeCalc{optByFlight: map[engineering.FlightType]*stair.OptimizeResult{
		engineering.FlightStraight: optResult(engineering.FlightStraight, 100, 630),
	}}
	retr := &fakeRetriever{chunks: []Chunk{
		pubChunk(0, "Прямой марш — рекомендация для малых высот.", "Прямой марш"),
		pubChunk(1, "Для винтовой лестницы нужен радиус.", "Винтовой марш"),
		pubChunk(2, "U-образный марш экономит место на площадке.", "U-образный марш"),
		pubChunk(3, "L-образный марш поворачивает на 90 градусов.", "L-образный марш"),
		pubChunk(4, "Шаг комфорта 2h+b в диапазоне 600–640 мм.", "Комфорт"),
	}}
	mem := &fakeMemory{}
	svc := NewService(calc, nil).WithRAG(retr, 5).WithMemory(mem).
		WithProjectAuthz(&fakeAuthz{members: map[string]bool{"t1/u1/p1": true}})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Ask(context.Background(), "t1", "u1", KindDesign, DesignRequest{
			Config:    stair.Config{Width: 900, Height: 2700, Flight: engineering.FlightStraight},
			ProjectID: "p1",
		}); err != nil {
			b.Fatalf("Ask: %v", err)
		}
	}
}
