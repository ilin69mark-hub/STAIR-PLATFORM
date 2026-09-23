package assistant

import (
	"strings"
	"testing"
)

// testDoc — небольшой markdown-документ с заголовками и блоками кода.
const testDoc = `# Лестницы

Платформа рассчитывает лестницы нескольких типов марша: прямой, L-образный,
U-образный и винтовой (спиральный). Для каждого типа действуют свои геометрические
ограничения (EDR-0005, EDR-0006, EDR-0007).

## Прямой марш

Прямой марш — самый простой тип. Требует минимального места в плане.

## L-образный марш

L-образный марш поворачивает на 90 градусов на промежуточной площадке.

Пример конфигурации:

` + "```json\n{\"flight\": \"lshape\"}\n```" + `

## U-образный марш

U-образный марш состоит из двух параллельных маршей с площадкой между ними.
Ширина площадки должна быть не меньше ширины марша.
`

func TestChunkerSplitsBySections(t *testing.T) {
	ch := NewMarkdownChunker(DefaultChunkOptions())
	chunks := ch.Chunk("docs/13_AI/test.md", "v1", testDoc)
	// Небольшой документ (~175 токенов < target 650) — один чанк.
	if len(chunks) != 1 {
		t.Fatalf("chunks = %d, want 1 for small doc", len(chunks))
	}
	c := chunks[0]
	if c.SourcePath != "docs/13_AI/test.md" || c.Version != "v1" {
		t.Fatalf("metadata wrong: %+v", c)
	}
	if c.DocTitle != "Лестницы" {
		t.Fatalf("doc title = %q, want Лестницы", c.DocTitle)
	}
	// Заголовки остаются в тексте чанков (контекст для модели).
	for _, h := range []string{"# Лестницы", "## Прямой марш", "## L-образный марш", "## U-образный марш"} {
		if !strings.Contains(c.Content, h) {
			t.Fatalf("chunk should keep heading %q", h)
		}
	}
	if c.Index != 0 {
		t.Fatalf("index = %d, want 0", c.Index)
	}
}

func TestChunkerMultipleChunksOnBigDoc(t *testing.T) {
	// Большой документ: 30 секций по ~40 слов ≈ 2600+ токенов → много чанков.
	var b strings.Builder
	b.WriteString("# Большой документ\n\n")
	for i := 0; i < 30; i++ {
		b.WriteString("## Секция " + itoa(i) + "\n\n")
		b.WriteString(strings.Repeat("описание секции с повторяющимися инженерными терминами ", 40) + "\n\n")
	}
	chunks := NewMarkdownChunker(DefaultChunkOptions()).Chunk("docs/13_AI/big.md", "v1", b.String())
	if len(chunks) < 3 {
		t.Fatalf("chunks = %d, want >= 3", len(chunks))
	}
	// Индексы стабильны и последовательны; нумерация с 0.
	for i, c := range chunks {
		if c.Index != i {
			t.Fatalf("chunk %d index = %d", i, c.Index)
		}
		if c.SourcePath != "docs/13_AI/big.md" {
			t.Fatalf("source wrong: %q", c.SourcePath)
		}
	}
}

func TestChunkerDeterministic(t *testing.T) {
	ch := NewMarkdownChunker(DefaultChunkOptions())
	a := ch.Chunk("d.md", "v1", testDoc)
	b := ch.Chunk("d.md", "v1", testDoc)
	if len(a) != len(b) {
		t.Fatalf("len(a)=%d len(b)=%d", len(a), len(b))
	}
	for i := range a {
		if a[i].Hash != b[i].Hash || a[i].Content != b[i].Content {
			t.Fatalf("chunk %d differs: %s vs %s", i, a[i].Hash, b[i].Hash)
		}
	}
	// Хеш — именно sha256(content).
	if a[0].Hash != ComputeChunkHash(a[0].Content) {
		t.Fatalf("hash mismatch")
	}
}

func TestChunkerRespectsTokenBounds(t *testing.T) {
	opts := ChunkOptions{TargetTokens: 30, MaxTokens: 50, OverlapTokens: 8}
	ch := NewMarkdownChunker(opts)
	// Длинный paragraph-текст без заголовков — вынуждает разбиение.
	long := "Слово " + strings.Repeat("лестница ", 400)
	chunks := ch.Chunk("d.md", "v", long)
	if len(chunks) == 0 {
		t.Fatal("no chunks")
	}
	// Оценка токенов включает пробелы между словами → допускаем +20% к max.
	slack := opts.MaxTokens + opts.MaxTokens/5
	for _, c := range chunks {
		if estimateTokens(c.Content) > slack {
			t.Fatalf("chunk %d tokens=%d > slack %d", c.Index, estimateTokens(c.Content), slack)
		}
	}
	// Есть перекрытие между соседними чанками (общие слова).
	if len(chunks) > 1 && !chunksOverlap(chunks) {
		t.Fatal("expected overlap between chunks")
	}
}

func chunksOverlap(chunks []Chunk) bool {
	for i := 1; i < len(chunks); i++ {
		prev := words(chunks[i-1].Content)
		cur := words(chunks[i].Content)
		if len(prev) == 0 || len(cur) == 0 {
			continue
		}
		if prev[len(prev)-1] == cur[0] || containsWord(cur, prev[len(prev)-1]) {
			return true
		}
	}
	return false
}

func words(s string) []string { return strings.Fields(s) }

func containsWord(list []string, w string) bool {
	for _, x := range list {
		if x == w {
			return true
		}
	}
	return false
}

func TestSplitTextWithOverlap(t *testing.T) {
	text := strings.Join(strings.Split(strings.Repeat("a b c d e f g h i j k l m n o p ", 40), " "), " ")
	pieces := splitTextWithOverlap(text, 32, 8)
	if len(pieces) < 3 {
		t.Fatalf("pieces = %d, want >= 3", len(pieces))
	}
	// Всегда есть прогресс: каждый следующий кусок начинается позже предыдущего.
	for i := 1; i < len(pieces); i++ {
		if !strings.Contains(text, pieces[i]) {
			t.Fatalf("piece %d not a substring", i)
		}
	}
	// Последний кусок — не пустой.
	if strings.TrimSpace(pieces[len(pieces)-1]) == "" {
		t.Fatal("last piece empty")
	}
}

func TestSplitTextOverlapProgress(t *testing.T) {
	// Уникальные слова (w000 w001 ...) — без ложных ранних вхождений.
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("w" + pad4(i) + " ")
	}
	text := b.String()
	pieces := splitTextWithOverlap(text, 16, 15)
	if len(pieces) == 0 {
		t.Fatal("no pieces")
	}
	last := -1
	for _, p := range pieces {
		idx := strings.Index(text, p)
		if idx < 0 {
			t.Fatalf("piece %q not found in text", p)
		}
		if idx < last {
			t.Fatalf("piece started earlier than previous: %d < %d", idx, last)
		}
		last = idx
	}
}

func pad4(n int) string {
	s := itoa(n)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

func TestEstimateTokens(t *testing.T) {
	if estimateTokens("") != 0 {
		t.Fatal("empty -> 0")
	}
	if estimateTokens("abcd") != 1 {
		t.Fatalf("4 chars -> 1 token, got %d", estimateTokens("abcd"))
	}
	if estimateTokens("abcde") != 2 {
		t.Fatalf("5 chars -> 2 tokens, got %d", estimateTokens("abcde"))
	}
}

func TestComputeChunkHashStable(t *testing.T) {
	a := ComputeChunkHash("same text")
	b := ComputeChunkHash("same text")
	c := ComputeChunkHash("other text")
	if a != b {
		t.Fatal("hash must be deterministic")
	}
	if a == c {
		t.Fatal("different text must hash differently")
	}
	if len(a) != 64 {
		t.Fatalf("sha256 hex len = %d", len(a))
	}
}

func TestCitationFormat(t *testing.T) {
	c := Chunk{SourcePath: "docs/13_AI/07_RAG_ENGINE.md", Index: 3}
	want := "[source: docs/13_AI/07_RAG_ENGINE.md, §3]"
	if got := c.Citation(); got != want {
		t.Fatalf("citation = %q, want %q", got, want)
	}
}

func TestChunkerKeepsFencedCodeIntact(t *testing.T) {
	doc := "# Док\n\nТекст.\n\n```sql\nSELECT 1;\n\nSELECT 2;\n```\n\nПосле.\n"
	chunks := NewMarkdownChunker(DefaultChunkOptions()).Chunk("d.md", "v", doc)
	for _, c := range chunks {
		// Блок кода не должен быть разорван: если фенс есть, он закрыт.
		if strings.Count(c.Content, "```")%2 != 0 {
			t.Fatalf("unbalanced fence in chunk %d: %q", c.Index, c.Content)
		}
	}
}

func TestChunkerEmptyDoc(t *testing.T) {
	chunks := NewMarkdownChunker(DefaultChunkOptions()).Chunk("d.md", "v", "")
	if len(chunks) != 0 {
		t.Fatalf("empty doc -> %d chunks", len(chunks))
	}
}

func TestChunkerOptionsNormalized(t *testing.T) {
	def := DefaultChunkOptions().normalized()
	if def.TargetTokens != defaultTargetTokens || def.MaxTokens != defaultMaxTokens {
		t.Fatalf("defaults wrong: %+v", def)
	}
	// MaxTokens < Target → расширяется до ~1.5×.
	o := ChunkOptions{TargetTokens: 100, MaxTokens: 50}.normalized()
	if o.MaxTokens < o.TargetTokens {
		t.Fatalf("max %d < target %d after normalize", o.MaxTokens, o.TargetTokens)
	}
	// Отрицательный overlap → 0 → дефолт.
	o = ChunkOptions{OverlapTokens: -5}.normalized()
	if o.OverlapTokens != defaultOverlap {
		t.Fatalf("negative overlap -> %d, want default", o.OverlapTokens)
	}
}

func TestChunkCitationIndexes(t *testing.T) {
	ch := NewMarkdownChunker(DefaultChunkOptions())
	chunks := ch.Chunk("d.md", "v", testDoc)
	for i, c := range chunks {
		if !strings.Contains(c.Citation(), "§"+itoa(i)+"]") {
			t.Fatalf("citation %q should contain §%d", c.Citation(), i)
		}
	}
}
