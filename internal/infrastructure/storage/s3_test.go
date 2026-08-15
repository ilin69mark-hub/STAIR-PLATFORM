package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestS3StorePutGetDelete — круговая проверка через httptest S3-сервер.
func TestS3StorePutGetDelete(t *testing.T) {
	var gotKey, gotCT, gotMethod string
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /bucket/", func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotKey = r.Method, r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		data, _ := io.ReadAll(r.Body)
		if string(data) != "payload" {
			t.Errorf("PUT body = %q", data)
		}
		if r.Header.Get("Authorization") == "" || !strings.Contains(r.Header.Get("Authorization"), "Signature=") {
			t.Errorf("no valid Authorization header: %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /bucket/", func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotKey = r.Method, r.URL.Path
		w.Write([]byte("payload"))
	})
	mux.HandleFunc("DELETE /bucket/", func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotKey = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	store, err := NewS3Store(Options{
		Endpoint: ts.URL, Bucket: "bucket", Region: "us-east-1",
		AccessKey: "AK", SecretKey: "SK", PathStyle: true,
	})
	if err != nil {
		t.Fatalf("NewS3Store: %v", err)
	}
	ctx := context.Background()

	if err := store.Put(ctx, "t/exports/cad/f.dxf", []byte("payload"), "application/dxf"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if gotMethod != "PUT" || gotKey != "/bucket/t/exports/cad/f.dxf" || gotCT != "application/dxf" {
		t.Fatalf("unexpected put: method=%s key=%s ct=%s", gotMethod, gotKey, gotCT)
	}

	data, err := store.Get(ctx, "t/exports/cad/f.dxf")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(data) != "payload" {
		t.Fatalf("Get data = %q", data)
	}
	if gotMethod != "GET" || gotKey != "/bucket/t/exports/cad/f.dxf" {
		t.Fatalf("unexpected get: method=%s key=%s", gotMethod, gotKey)
	}

	if err := store.Delete(ctx, "t/exports/cad/f.dxf"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotMethod != "DELETE" || gotKey != "/bucket/t/exports/cad/f.dxf" {
		t.Fatalf("unexpected delete: method=%s key=%s", gotMethod, gotKey)
	}
}

// TestS3StoreNotFound — 404 от сервера → ErrNotFound.
func TestS3StoreNotFound(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()
	store, err := NewS3Store(Options{
		Endpoint: ts.URL, Bucket: "bucket", AccessKey: "AK", SecretKey: "SK",
	})
	if err != nil {
		t.Fatalf("NewS3Store: %v", err)
	}
	if _, err := store.Get(context.Background(), "t/f"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get: err = %v, want ErrNotFound", err)
	}
}

// TestS3StoreInvalidKey — ключ с ".." отклоняется до HTTP.
func TestS3StoreInvalidKey(t *testing.T) {
	store, err := NewS3Store(Options{
		Endpoint: "http://localhost:9000", Bucket: "b", AccessKey: "AK", SecretKey: "SK",
	})
	if err != nil {
		t.Fatalf("NewS3Store: %v", err)
	}
	if err := store.Put(context.Background(), "../evil", []byte("x"), ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Put: err = %v, want ErrInvalid", err)
	}
}

// TestNewObjectStoreFactory — выбор бэкенда по имени.
func TestNewObjectStoreFactory(t *testing.T) {
	o := Options{FSRoot: t.TempDir()}
	if _, err := NewObjectStore("filesystem", o); err != nil {
		t.Fatalf("filesystem: %v", err)
	}
	if _, err := NewObjectStore("", o); err != nil {
		t.Fatalf("default: %v", err)
	}
	if _, err := NewObjectStore("s3", Options{Endpoint: "http://x", Bucket: "b", AccessKey: "A", SecretKey: "S"}); err != nil {
		t.Fatalf("s3: %v", err)
	}
	if _, err := NewObjectStore("nope", o); err == nil {
		t.Fatal("unknown backend must error")
	}
}
