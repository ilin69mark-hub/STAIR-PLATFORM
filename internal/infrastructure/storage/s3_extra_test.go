package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewS3StoreValidation(t *testing.T) {
	cases := []Options{
		{},
		{Endpoint: "http://x", AccessKey: "A", SecretKey: "S"},
		{Endpoint: "http://x", Bucket: "b"},
	}
	for _, o := range cases {
		if _, err := NewS3Store(o); !errors.Is(err, ErrInvalid) {
			t.Fatalf("NewS3Store(%+v) = %v, want ErrInvalid", o, err)
		}
	}
}

func TestS3PutInvalidEndpoint(t *testing.T) {
	store, err := NewS3Store(Options{
		Endpoint: "http://ex%zzam.com", Bucket: "b", AccessKey: "A", SecretKey: "S",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "k", []byte("x"), ""); err == nil {
		t.Fatal("expected parse url error")
	}
}

func TestS3PutConnectionError(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	u := ts.URL
	ts.Close()
	store, err := NewS3Store(Options{Endpoint: u, Bucket: "b", AccessKey: "A", SecretKey: "S"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "k", []byte("x"), ""); err == nil {
		t.Fatal("expected request error")
	}
}

func TestS3PutNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()
	store, err := NewS3Store(Options{Endpoint: ts.URL, Bucket: "b", AccessKey: "A", SecretKey: "S"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "k", []byte("x"), ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Put = %v, want ErrNotFound", err)
	}
}

func TestS3PutServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `<Error><Code>AccessDenied</Code><Message>no</Message></Error>`)
	}))
	defer ts.Close()
	store, err := NewS3Store(Options{Endpoint: ts.URL, Bucket: "b", AccessKey: "A", SecretKey: "S"})
	if err != nil {
		t.Fatal(err)
	}
	err = store.Put(context.Background(), "k", []byte("x"), "")
	if err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalid) {
		t.Fatalf("Put = %v, want generic s3 error", err)
	}
}

func TestS3DeleteInvalidKey(t *testing.T) {
	store, err := NewS3Store(Options{
		Endpoint: "http://localhost:9000", Bucket: "b", AccessKey: "A", SecretKey: "S",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), "../evil"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Delete = %v, want ErrInvalid", err)
	}
}

func TestS3DeleteNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer ts.Close()
	store, err := NewS3Store(Options{Endpoint: ts.URL, Bucket: "b", AccessKey: "A", SecretKey: "S"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete = %v, want ErrNotFound", err)
	}
}

func TestS3DeleteServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer ts.Close()
	store, err := NewS3Store(Options{Endpoint: ts.URL, Bucket: "b", AccessKey: "A", SecretKey: "S"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), "k"); err == nil {
		t.Fatal("expected generic error")
	}
}

func TestS3GetInvalidKey(t *testing.T) {
	store, err := NewS3Store(Options{
		Endpoint: "http://localhost:9000", Bucket: "b", AccessKey: "A", SecretKey: "S",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), "../evil"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Get = %v, want ErrInvalid", err)
	}
}

func TestS3GetInvalidEndpoint(t *testing.T) {
	store, err := NewS3Store(Options{
		Endpoint: "http://ex%zzam.com", Bucket: "b", AccessKey: "A", SecretKey: "S",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatal("expected parse url error")
	}
}

func TestS3GetConnectionError(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	u := ts.URL
	ts.Close()
	store, err := NewS3Store(Options{Endpoint: u, Bucket: "b", AccessKey: "A", SecretKey: "S"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatal("expected request error")
	}
}

func TestS3GetServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `<Error><Code>AccessDenied</Code><Message>no</Message></Error>`)
	}))
	defer ts.Close()
	store, err := NewS3Store(Options{Endpoint: ts.URL, Bucket: "b", AccessKey: "A", SecretKey: "S"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatal("expected generic error")
	}
}

func TestS3GetTruncatedBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("short"))
	}))
	defer ts.Close()
	store, err := NewS3Store(Options{Endpoint: ts.URL, Bucket: "b", AccessKey: "A", SecretKey: "S"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatal("expected read body error")
	}
}
