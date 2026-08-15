package storage

import (
	"strings"
	"testing"
	"time"
)

// TestSigV4AWSTestVector — известный эталон из документации AWS
// (GET https://iam.amazonaws.com/?Action=ListUsers&Version=2010-05-08).
func TestSigV4AWSTestVector(t *testing.T) {
	s := newSigV4Signer("AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		"us-east-1", "iam")
	now := time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC)
	headers := "content-type:application/x-www-form-urlencoded; charset=utf-8\nhost:iam.amazonaws.com\nx-amz-date:20150830T123600Z\n"
	signed := "content-type;host;x-amz-date"
	payloadHash := hexSHA256(nil)
	auth := s.sign("GET", "/", "Action=ListUsers&Version=2010-05-08", now,
		signed, headers, payloadHash)
	if !strings.Contains(auth, "Signature=5d672d79c15b13162d9279b0855cfba6789a8edb4c82c400e06b5924a6f2b5d7") {
		t.Fatalf("unexpected signature %q", auth)
	}
}

// TestSigV4CanonicalQuery — сортировка и кодирование query.
func TestSigV4CanonicalQuery(t *testing.T) {
	got := canonicalQuery("Version=2010-05-08&Action=ListUsers&Z=1&a=2")
	want := "Action=ListUsers&Version=2010-05-08&Z=1&a=2"
	if got != want {
		t.Fatalf("canonicalQuery = %q, want %q", got, want)
	}
}

// TestSigV4UriEncode — кодирование сегментов пути.
func TestSigV4UriEncode(t *testing.T) {
	got := uriEncode("/a b/c+d")
	if got != "/a%20b/c%2Bd" {
		t.Fatalf("uriEncode = %q", got)
	}
}

// TestSigV4Deterministic — подпись детерминирована при фиксированном времени.
func TestSigV4Deterministic(t *testing.T) {
	s := newSigV4Signer("k", "secret", "us-east-1", "s3")
	now := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	hdr := "host:example.com\nx-amz-content-sha256:abc\nx-amz-date:20260815T100000Z\n"
	a := s.sign("PUT", "/bucket/key", "", now, "host;x-amz-content-sha256;x-amz-date", hdr, "abc")
	b := s.sign("PUT", "/bucket/key", "", now, "host;x-amz-content-sha256;x-amz-date", hdr, "abc")
	if a != b {
		t.Fatalf("signature not deterministic: %q vs %q", a, b)
	}
}
