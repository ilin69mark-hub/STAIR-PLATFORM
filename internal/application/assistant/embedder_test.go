package assistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeEmbedServer — OpenAI-совместимый /embeddings на httptest.
type fakeEmbedServer struct {
	mu       sync.Mutex
	srv      *httptest.Server
	dim      int
	batches  []int // размеры пришедших батчей
	auth     string
	failCode int
	model    string
}

func newFakeEmbedServer(dim int) *fakeEmbedServer {
	f := &fakeEmbedServer{dim: dim, model: "test-embed"}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	return f
}

func (f *fakeEmbedServer) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/embeddings" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	f.auth = r.Header.Get("Authorization")
	var body struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.batches = append(f.batches, len(body.Input))
	if f.failCode != 0 {
		w.WriteHeader(f.failCode)
		_, _ = w.Write([]byte("embed boom"))
		return
	}
	data := make([]map[string]interface{}, len(body.Input))
	for i, text := range body.Input {
		vec := make([]float32, f.dim)
		// Детерминированный псевдовектор на основе текста (для тестов косинуса).
		for j := 0; j < f.dim; j++ {
			vec[j] = float32(int(text[0])%7+1) / float32(j+1)
		}
		data[i] = map[string]interface{}{
			"index":     i,
			"embedding": vec,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
}

func (f *fakeEmbedServer) close() { f.srv.Close() }

func (f *fakeEmbedServer) batchSizes() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.batches...)
}

func TestOpenAIEmbedderSuccess(t *testing.T) {
	s := newFakeEmbedServer(EmbeddingDim)
	defer s.close()
	emb, err := NewOpenAIEmbedder(EmbedderConfig{BaseURL: s.srv.URL, APIKey: "k", Model: "m"})
	if err != nil {
		t.Fatalf("NewOpenAIEmbedder: %v", err)
	}
	vecs, err := emb.Embed(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 3 {
		t.Fatalf("vectors = %d, want 3", len(vecs))
	}
	for i, v := range vecs {
		if len(v) != EmbeddingDim {
			t.Fatalf("vec %d dim = %d", i, len(v))
		}
	}
	s.mu.Lock()
	auth := s.auth
	s.mu.Unlock()
	if auth != "Bearer k" {
		t.Fatalf("auth = %q", auth)
	}
}

func TestOpenAIEmbedderBatching(t *testing.T) {
	s := newFakeEmbedServer(EmbeddingDim)
	defer s.close()
	emb, _ := NewOpenAIEmbedder(EmbedderConfig{BaseURL: s.srv.URL, Model: "m"})
	n := 130 // > batchSize → несколько запросов
	texts := make([]string, n)
	for i := range texts {
		texts[i] = "text-" + strconv.Itoa(i)
	}
	vecs, err := emb.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != n {
		t.Fatalf("vectors = %d, want %d", len(vecs), n)
	}
	batches := s.batchSizes()
	if len(batches) != 3 { // 64 + 64 + 2
		t.Fatalf("batches = %v, want [64 64 2]", batches)
	}
}

func TestOpenAIEmbedderRequiresBaseURLAndModel(t *testing.T) {
	if _, err := NewOpenAIEmbedder(EmbedderConfig{Model: "m"}); err == nil {
		t.Fatal("expected error without BaseURL")
	}
	if _, err := NewOpenAIEmbedder(EmbedderConfig{BaseURL: "http://x"}); err == nil {
		t.Fatal("expected error without Model")
	}
}

func TestOpenAIEmbedderErrorStatus(t *testing.T) {
	s := newFakeEmbedServer(EmbeddingDim)
	s.failCode = http.StatusInternalServerError
	defer s.close()
	emb, _ := NewOpenAIEmbedder(EmbedderConfig{BaseURL: s.srv.URL, Model: "m"})
	if _, err := emb.Embed(context.Background(), []string{"x"}); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestOpenAIEmbedderWrongDim(t *testing.T) {
	s := newFakeEmbedServer(EmbeddingDim - 1)
	defer s.close()
	emb, _ := NewOpenAIEmbedder(EmbedderConfig{BaseURL: s.srv.URL, Model: "m"})
	_, err := emb.Embed(context.Background(), []string{"x"})
	if err == nil || !strings.Contains(err.Error(), "1536") {
		t.Fatalf("err = %v, want dimension mismatch mentioning 1536", err)
	}
}

func TestOpenAIEmbedderMissingData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data": []}`))
	}))
	defer srv.Close()
	emb, _ := NewOpenAIEmbedder(EmbedderConfig{BaseURL: srv.URL, Model: "m"})
	if _, err := emb.Embed(context.Background(), []string{"x", "y"}); err == nil {
		t.Fatal("expected count mismatch error")
	}
}

func TestOpenAIEmbedderCustomHeaders(t *testing.T) {
	s := newFakeEmbedServer(EmbeddingDim)
	defer s.close()
	emb, _ := NewOpenAIEmbedder(EmbedderConfig{
		BaseURL: s.srv.URL,
		Model:   "m",
		Headers: map[string]string{"HTTP-Referer": "https://example.com", "": "skip-me"},
	})
	if _, err := emb.Embed(context.Background(), []string{"x"}); err != nil {
		t.Fatalf("Embed: %v", err)
	}
}
