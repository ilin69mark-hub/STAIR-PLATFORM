package assistant

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// ChunkOptions — параметры чанкера markdown-документов (AI-0007).
// Размеры в «токенах»: точный токенизатор не используется (детерминизм и
// нулевые зависимости) — оценка: длина текста в рунах / 4 (эмпирика
// сопоставима с ~4 символами на токен для EN/RU-прозы).
type ChunkOptions struct {
	// TargetTokens — целевой размер чанка; 0 → 650 (диапазон 500–800).
	TargetTokens int
	// MaxTokens — жёсткий верх чанка; меньше Target → ≈1.4×Target (≤800).
	MaxTokens int
	// OverlapTokens — перекрытие между чанками; 0 → 100.
	OverlapTokens int
}

// Параметры по умолчанию: чанки ~500–800 токенов с перекрытием ~100 (S-135).
const (
	defaultTargetTokens = 650
	defaultMaxTokens    = 800
	defaultOverlap      = 100
)

// DefaultChunkOptions — параметры чанкера по умолчанию.
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{
		TargetTokens:  defaultTargetTokens,
		MaxTokens:     defaultMaxTokens,
		OverlapTokens: defaultOverlap,
	}
}

func (o ChunkOptions) normalized() ChunkOptions {
	if o.TargetTokens <= 0 {
		o.TargetTokens = defaultTargetTokens
	}
	if o.MaxTokens < o.TargetTokens {
		o.MaxTokens = o.TargetTokens + o.TargetTokens/2 // ≈1.5×Target
	}
	if o.MaxTokens <= 0 {
		o.MaxTokens = defaultMaxTokens
	}
	if o.OverlapTokens < 0 {
		o.OverlapTokens = 0
	}
	if o.OverlapTokens == 0 {
		o.OverlapTokens = defaultOverlap
	}
	return o
}

// Chunk — фрагмент документа публичного корпуса RAG (AI-0007). Корпус
// публичный (tenant_id = NULL): конфиги пользователей в индекс не
// складываются, tenant-фильтр применяется в запросах ретривера (SEC-0005).
type Chunk struct {
	// Hash — sha256(Content), ключ идемпотентного backfill.
	Hash       string
	SourcePath string // путь документа (docs/13_AI/07_RAG_ENGINE.md)
	Version    string // версия корпуса (git ref/дата)
	DocTitle   string // ближайший H1/H2-заголовок (для метаданных/цитат)
	Index      int    // порядковый номер чанка в документе (с 0)
	Content    string
	// Embedding — вектор чанка (прикладной слой не обязан его хранить);
	// заполняется backfill'ом для записи в embedding vector(1536) и
	// возвращается ретривером вместе с чанком (S-135).
	Embedding []float32
}

// ComputeChunkHash — sha256(content) в hex. Детерминирован: одинаковый текст
// всегда даёт одинаковый ключ идемпотентного backfill (S-135).
func ComputeChunkHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// Citation — формат цитаты источника в Response.Notes: [source: путь, §N].
func (c Chunk) Citation() string {
	return "[source: " + c.SourcePath + ", §" + itoa(c.Index) + "]"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// markdownHeading распознаёт заголовок ATX (# .. ######).
var markdownHeading = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)

// mdBlock — единица разбора: абзац, заголовок или блок кода.
type mdBlock struct {
	text        string
	isHeading   bool
	headingPath []string // путь заголовков на момент блока (контекст)
	title       string   // ближайший H1/H2 для метаданных чанка
}

// MarkdownChunker — чанкер markdown-документов корпуса (S-135): разбирает
// документ на блоки (заголовки/абзацы/фенсы кода), собирает блоки в чанки
// целевого размера. Перекрытие — «хвост» предыдущего чанка в начале
// следующего при закрытии по размеру внутри одной секции, а также окна
// splitTextWithOverlap для сверхдлинных блоков. Детерминирован.
type MarkdownChunker struct {
	opts ChunkOptions
}

// NewMarkdownChunker создаёт чанкер; нулевые поля опций — по умолчанию.
func NewMarkdownChunker(opts ChunkOptions) *MarkdownChunker {
	return &MarkdownChunker{opts: opts.normalized()}
}

// splitBlocks — разбор markdown на блоки с фиксацией путей заголовков.
// Блок кода (``` ... ```) не рвётся на границах абзацев внутри фенса.
func splitBlocks(doc string) []mdBlock {
	doc = strings.ReplaceAll(doc, "\r\n", "\n")
	lines := strings.Split(doc, "\n")

	var (
		blocks   []mdBlock
		path     []string // стек заголовков (путь секции)
		levels   [7]int   // позиция заголовка уровня N в path (-1 = нет)
		buf      []string
		inFence  bool
		fence    string
		curTitle string
	)
	for i := range levels {
		levels[i] = -1
	}
	flush := func() {
		if len(buf) == 0 {
			return
		}
		text := strings.TrimRight(strings.Join(buf, "\n"), " \t")
		if text != "" {
			blocks = append(blocks, mdBlock{
				text:        text,
				headingPath: append([]string(nil), path...),
				title:       curTitle,
			})
		}
		buf = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Открытие/закрытие фенса кода.
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			if !inFence {
				flush()
				inFence = true
				fence = trimmed[:3]
				buf = append(buf, line)
				continue
			}
			if strings.HasPrefix(trimmed, fence) {
				buf = append(buf, line)
				flush()
				inFence = false
				fence = ""
				continue
			}
		}
		if inFence {
			buf = append(buf, line)
			continue
		}
		if m := markdownHeading.FindStringSubmatch(line); m != nil {
			flush()
			level := len(m[1])
			title := strings.TrimSpace(m[2])
			// Уровень приходит «сверху вниз»: сбрасываем более глубокие.
			for lv := level; lv <= 6; lv++ {
				if pos := levels[lv]; pos >= 0 && pos < len(path) {
					path = path[:pos]
				}
				levels[lv] = -1
			}
			// Стек до уровня level-1, затем сам заголовок.
			depth := level - 1
			if depth < len(path) {
				path = path[:depth]
			}
			path = append(path, title)
			levels[level] = len(path) - 1
			if level <= 2 {
				curTitle = title
			}
			blocks = append(blocks, mdBlock{
				text:        line,
				isHeading:   true,
				headingPath: append([]string(nil), path...),
				title:       curTitle,
			})
			continue
		}
		if trimmed == "" {
			flush() // граница абзацев
			continue
		}
		buf = append(buf, line)
	}
	flush()
	if inFence && len(buf) > 0 { // незакрытый фенс — считаем блоком
		blocks = append(blocks, mdBlock{
			text:        strings.Join(buf, "\n"),
			headingPath: append([]string(nil), path...),
			title:       curTitle,
		})
	}
	return blocks
}

// estimateTokens — оценка числа токенов: руны / 4 (см. ChunkOptions).
func estimateTokens(s string) int {
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	t := n / 4
	if t*4 < n {
		t++
	}
	return t
}

// splitTextWithOverlap — разбиение сверхдлинного текста на части с оценкой
// ≤ maxTokens каждая и перекрытием overlapTokens слов (по пробелам).
// Окно накапливается по ОЦЕНКЕ токенов (а не по счётчику слов): иначе
// длинные слова (кириллица, URL) давали бы чанки в 2–3× больше лимита.
// Всегда прогрессирует (start строго растёт).
func splitTextWithOverlap(text string, maxTokens, overlapTokens int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	if estimateTokens(text) <= maxTokens {
		return []string{text}
	}
	overlap := overlapTokens
	if overlap < 1 {
		overlap = 1
	}
	var out []string
	for start := 0; start < len(words); {
		// Накапливаем слова, пока оценка токенов не упрётся в maxTokens.
		end := start
		acc := 0
		for end < len(words) {
			t := estimateTokens(words[end])
			if end > start && acc+t > maxTokens {
				break
			}
			acc += t
			end++
		}
		if end == start { // защита: одиночное «слово» длиннее лимита
			end = start + 1
		}
		out = append(out, strings.Join(words[start:end], " "))
		if end >= len(words) {
			break
		}
		next := end - overlap
		if next <= start { // гарантия прогресса окна
			next = end
		}
		start = next
	}
	return out
}

// tailWords — последние n слов текста (для перекрытия чанков).
func tailWords(text string, n int) []string {
	if n <= 0 {
		return nil
	}
	words := strings.Fields(text)
	if len(words) <= n {
		return append([]string(nil), words...)
	}
	return append([]string(nil), words[len(words)-n:]...)
}

func samePath(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// appendBlock добавляет блок в текущий чанк.
func appendBlock(cur *strings.Builder, curTok *int, prefix, text string) {
	if cur.Len() > 0 {
		cur.WriteString("\n\n")
	}
	cur.WriteString(prefix)
	cur.WriteString(text)
	*curTok += estimateTokens(prefix) + estimateTokens(text)
}

// Chunk разбирает документ на чанки корпуса со стабильной нумерацией.
// sourcePath/version — метаданные; Hash = sha256(content).
func (c *MarkdownChunker) Chunk(sourcePath, version, doc string) []Chunk {
	opts := c.opts.normalized()
	blocks := splitBlocks(doc)
	if len(blocks) == 0 {
		return nil
	}

	var (
		out      []Chunk
		cur      strings.Builder
		curTok   int
		curTitle string
		lastTail []string // хвост последнего закрытого чанка (перекрытие)
		lastPath []string
	)

	emit := func(content, title string) {
		content = strings.TrimSpace(content)
		if content == "" {
			return
		}
		out = append(out, Chunk{
			Hash:       ComputeChunkHash(content),
			SourcePath: sourcePath,
			Version:    version,
			DocTitle:   title,
			Index:      len(out),
			Content:    content,
		})
	}

	flush := func() {
		if cur.Len() == 0 {
			return
		}
		emit(cur.String(), curTitle)
		cur.Reset()
		curTok = 0
		curTitle = ""
	}

	// perPathTail перекрытие для блока: хвост предыдущего чанка добавляем
	// только внутри той же секции и не перед заголовком.
	prefixFor := func(b mdBlock) string {
		if len(lastTail) == 0 || b.isHeading {
			return ""
		}
		if !samePath(lastPath, b.headingPath) {
			return ""
		}
		return strings.Join(lastTail, " ") + " "
	}

	closeChunk := func(b mdBlock) {
		flush()
		if len(out) == 0 {
			return
		}
		lastTail = tailWords(out[len(out)-1].Content, opts.OverlapTokens)
		lastPath = b.headingPath
	}

	newChunk := func(prefix string, b mdBlock) {
		if prefix != "" && estimateTokens(prefix+b.text) > opts.MaxTokens {
			prefix = "" // перекрытие не влезает — отбрасываем
		}
		appendBlock(&cur, &curTok, prefix, b.text)
		if curTitle == "" {
			curTitle = b.title
		}
	}

	for i := 0; i < len(blocks); i++ {
		b := blocks[i]
		// Сверхдлинный блок: закрываем текущий чанк, режем блок отдельно.
		if estimateTokens(b.text) > opts.MaxTokens {
			closeChunk(b)
			for _, piece := range splitTextWithOverlap(b.text, opts.MaxTokens, opts.OverlapTokens) {
				emit(piece, b.title)
			}
			lastTail = tailWords(out[len(out)-1].Content, opts.OverlapTokens)
			lastPath = b.headingPath
			continue
		}

		// Перекрытие добавляем ТОЛЬКО в начало нового чанка (newChunk);
		// в уже накопленный чанк дублировать хвост нельзя.
		if curTok+estimateTokens(b.text) <= opts.MaxTokens {
			// Title фиксируем по ПЕРВОМУ блоку чанка (заголовок секции,
			// где чанк начинается) — не перезаписываем последующими.
			if cur.Len() == 0 {
				curTitle = b.title
			}
			appendBlock(&cur, &curTok, "", b.text)
			continue
		}
		// Не влезает: закрываем чанк; следующий начнётся с перекрытия.
		closeChunk(b)
		newChunk(prefixFor(b), b)
	}
	flush()
	return out
}
