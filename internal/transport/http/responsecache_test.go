package http

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestResponseCacheMiddleware_CachesGET(t *testing.T) {
	var callCount atomic.Int32
	handler := ResponseCacheMiddleware(ResponseCacheConfig{
		MaxEntries: 10,
		DefaultTTL: 1 * time.Minute,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":"test"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Cache") != "MISS" {
		t.Errorf("expected X-Cache MISS, got %q", w.Header().Get("X-Cache"))
	}

	// Second request should be cached
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)

	if w2.Header().Get("X-Cache") != "HIT" {
		t.Errorf("expected X-Cache HIT, got %q", w2.Header().Get("X-Cache"))
	}
	if callCount.Load() != 1 {
		t.Errorf("expected handler called once, got %d", callCount.Load())
	}
}

func TestResponseCacheMiddleware_NoCachePOST(t *testing.T) {
	var callCount atomic.Int32
	handler := ResponseCacheMiddleware(ResponseCacheConfig{
		MaxEntries: 10,
		DefaultTTL: 1 * time.Minute,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Cache") != "" {
		t.Errorf("expected no X-Cache header for POST, got %q", w.Header().Get("X-Cache"))
	}

	// Second POST should also call handler
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)

	if callCount.Load() != 2 {
		t.Errorf("expected handler called twice, got %d", callCount.Load())
	}
}

func TestResponseCacheMiddleware_NoCacheErrors(t *testing.T) {
	var callCount atomic.Int32
	handler := ResponseCacheMiddleware(ResponseCacheConfig{
		MaxEntries: 10,
		DefaultTTL: 1 * time.Minute,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)

	if callCount.Load() != 2 {
		t.Errorf("expected handler called twice for errors, got %d", callCount.Load())
	}
}

func TestResponseCacheMiddleware_NoCacheIdentityRequests(t *testing.T) {
	// Авторизованные запросы (session-cookie / Bearer) не должны ни читать,
	// ни заполнять кэш: ответы зависят от субъекта.
	for name, setAuth := range map[string]func(*http.Request){
		"Cookie": func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "t"}) //nolint:gosec // G124: тестовый cookie
		},
		"CSRF": func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "c"}) //nolint:gosec // G124: тестовый cookie
		},
		"Bearer": func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer key")
		},
	} {
		// Заполняем кэш АНОНИМНЫМ ответом.
		var anonCalls, idCalls atomic.Int32
		handler := ResponseCacheMiddleware(ResponseCacheConfig{
			MaxEntries: 10,
			DefaultTTL: 1 * time.Minute,
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" || len(r.Cookies()) > 0 {
				idCalls.Add(1)
				_, _ = w.Write([]byte("personal"))
				return
			}
			anonCalls.Add(1)
			_, _ = w.Write([]byte("public"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Header().Get("X-Cache") != "MISS" {
			t.Errorf("[%s] expected anonymous MISS, got %q", name, w.Header().Get("X-Cache"))
		}

		// Identity-запрос ДОЛЖЕН пройти в handler, а не получить кэшированный
		// анонимный ответ.
		idReq := httptest.NewRequest(http.MethodGet, "/test", nil)
		setAuth(idReq)
		idW := httptest.NewRecorder()
		handler.ServeHTTP(idW, idReq)

		if idW.Header().Get("X-Cache") != "" {
			t.Errorf("[%s] identity request hit cache, X-Cache=%q", name, idW.Header().Get("X-Cache"))
		}
		if got := idW.Body.String(); got != "personal" {
			t.Errorf("[%s] expected handler executed, got body %q", name, got)
		}
		if anonCalls.Load() != 1 {
			t.Errorf("[%s] expected anonymous handler once, got %d", name, anonCalls.Load())
		}
		if idCalls.Load() != 1 {
			t.Errorf("[%s] expected identity handler once, got %d", name, idCalls.Load())
		}
	}
}

func TestResponseCacheMiddleware_NoCacheSetCookieResponse(t *testing.T) {
	var callCount atomic.Int32
	handler := ResponseCacheMiddleware(ResponseCacheConfig{
		MaxEntries: 10,
		DefaultTTL: 1 * time.Minute,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: "t", Path: "/"}) //nolint:gosec // G124: тестовый cookie, без реальных атрибутов безопасности
		_, _ = w.Write([]byte("session"))
	}))

	// Первый запрос выполнился (Set-Cookie), но в кэш не попал.
	req := httptest.NewRequest(http.MethodGet, "/login-check", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Второй с идентичным запросом — handler обязан выполниться снова.
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)

	if callCount.Load() != 2 {
		t.Errorf("expected handler called twice (Set-Cookie not cached), got %d", callCount.Load())
	}
	if w2.Header().Get("X-Cache") == "HIT" {
		t.Errorf("expected no X-Cache HIT for Set-Cookie response")
	}
}

func TestResponseCacheMiddleware_TTLExpiration(t *testing.T) {
	var callCount atomic.Int32
	handler := ResponseCacheMiddleware(ResponseCacheConfig{
		MaxEntries: 10,
		DefaultTTL: 50 * time.Millisecond,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	time.Sleep(100 * time.Millisecond)

	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)

	if callCount.Load() != 2 {
		t.Errorf("expected handler called twice after TTL, got %d", callCount.Load())
	}
}

func TestResponseCacheMiddleware_DifferentPaths(t *testing.T) {
	var callCount atomic.Int32
	handler := ResponseCacheMiddleware(ResponseCacheConfig{
		MaxEntries: 10,
		DefaultTTL: 1 * time.Minute,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		_, _ = w.Write([]byte("ok"))
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/path1", nil)
	req2 := httptest.NewRequest(http.MethodGet, "/path2", nil)

	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if callCount.Load() != 2 {
		t.Errorf("expected handler called twice for different paths, got %d", callCount.Load())
	}
}

func TestResponseCacheMiddleware_Eviction(t *testing.T) {
	handler := ResponseCacheMiddleware(ResponseCacheConfig{
		MaxEntries: 2,
		DefaultTTL: 1 * time.Minute,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/path", nil)
		q := req.URL.Query()
		q.Set("page", string(rune('0'+i)))
		req.URL.RawQuery = q.Encode()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func TestResponseCacheMiddleware_QueryStringDiff(t *testing.T) {
	var callCount atomic.Int32
	handler := ResponseCacheMiddleware(ResponseCacheConfig{
		MaxEntries: 10,
		DefaultTTL: 1 * time.Minute,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		_, _ = w.Write([]byte("ok"))
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/test?page=1", nil)
	req2 := httptest.NewRequest(http.MethodGet, "/test?page=2", nil)

	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if callCount.Load() != 2 {
		t.Errorf("expected handler called twice for different query strings, got %d", callCount.Load())
	}
}

func TestResponseCacheEntry(t *testing.T) {
	cache := &responseCache{
		entries: make(map[string]*ResponseCacheEntry),
		config: ResponseCacheConfig{
			MaxEntries: 10,
			DefaultTTL: 1 * time.Minute,
		},
	}

	entry := &ResponseCacheEntry{
		Body:       []byte("test"),
		StatusCode: 200,
		Headers:    http.Header{"Content-Type": []string{"text/plain"}},
		ExpiresAt:  time.Now().Add(1 * time.Minute),
	}
	cache.Set("key1", entry)

	got, ok := cache.Get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(got.Body) != "test" {
		t.Errorf("expected body 'test', got %q", string(got.Body))
	}
	if cache.Size() != 1 {
		t.Errorf("expected size 1, got %d", cache.Size())
	}
}

func TestResponseCacheEntry_Expiry(t *testing.T) {
	cache := &responseCache{
		entries: make(map[string]*ResponseCacheEntry),
		config: ResponseCacheConfig{
			MaxEntries: 10,
			DefaultTTL: 1 * time.Minute,
		},
	}

	entry := &ResponseCacheEntry{
		Body:       []byte("test"),
		StatusCode: 200,
		ExpiresAt:  time.Now().Add(-1 * time.Second), // Already expired
	}
	cache.Set("key1", entry)

	_, ok := cache.Get("key1")
	if ok {
		t.Error("expected cache miss for expired entry")
	}
}
