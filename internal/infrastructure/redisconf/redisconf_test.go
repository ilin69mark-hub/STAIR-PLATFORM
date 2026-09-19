package redisconf

import (
	"strings"
	"testing"
)

func TestFromEnvPlainAddr(t *testing.T) {
	t.Setenv("STAIR_REDIS_PASSWORD", "")
	t.Setenv("STAIR_REDIS_DB", "")
	t.Setenv("STAIR_REDIS_TLS", "")
	opts := FromEnv("redis:6379")
	if opts.Addr != "redis:6379" {
		t.Fatalf("Addr = %q, want redis:6379", opts.Addr)
	}
	if opts.Password != "" || opts.DB != 0 || opts.TLSConfig != nil {
		t.Fatalf("unexpected extras: %+v", opts)
	}
}

func TestFromEnvURLScheme(t *testing.T) {
	t.Setenv("STAIR_REDIS_PASSWORD", "")
	t.Setenv("STAIR_REDIS_DB", "")
	t.Setenv("STAIR_REDIS_TLS", "")
	opts := FromEnv("redis://redis:6379")
	if opts.Addr != "redis:6379" {
		t.Fatalf("Addr = %q, want redis:6379", opts.Addr)
	}
}

func TestFromEnvURLWithPasswordAndDB(t *testing.T) {
	t.Setenv("STAIR_REDIS_PASSWORD", "envpass")
	t.Setenv("STAIR_REDIS_DB", "7")
	t.Setenv("STAIR_REDIS_TLS", "")
	opts := FromEnv("redis://:urlpass@redis.example:6380/3")
	if opts.Addr != "redis.example:6380" {
		t.Fatalf("Addr = %q", opts.Addr)
	}
	if opts.Password != "urlpass" {
		t.Fatalf("Password = %q, want urlpass (URL приоритетнее env)", opts.Password)
	}
	if opts.DB != 3 {
		t.Fatalf("DB = %d, want 3", opts.DB)
	}
}

func TestFromEnvURLWithoutPasswordUsesEnv(t *testing.T) {
	t.Setenv("STAIR_REDIS_PASSWORD", "envpass")
	t.Setenv("STAIR_REDIS_DB", "")
	t.Setenv("STAIR_REDIS_TLS", "")
	opts := FromEnv("redis://redis:6379/2")
	if opts.Password != "envpass" {
		t.Fatalf("Password = %q, want envpass", opts.Password)
	}
	if opts.DB != 2 {
		t.Fatalf("DB = %d, want 2", opts.DB)
	}
}

func TestFromEnvRedisPassIsTLS(t *testing.T) {
	t.Setenv("STAIR_REDIS_TLS", "")
	opts := FromEnv("rediss://:pw@redis:6379")
	if opts.Addr != "redis:6379" {
		t.Fatalf("Addr = %q", opts.Addr)
	}
	if opts.Password != "pw" {
		t.Fatalf("Password = %q", opts.Password)
	}
	if opts.TLSConfig == nil {
		t.Fatal("TLSConfig = nil, rediss:// должен включать TLS")
	}
}

func TestFromEnvTLSFlag(t *testing.T) {
	t.Setenv("STAIR_REDIS_TLS", "true")
	opts := FromEnv("redis:6379")
	if opts.TLSConfig == nil {
		t.Fatal("TLSConfig = nil, STAIR_REDIS_TLS=true должен включать TLS")
	}
}

func TestFromEnvEnvDBWhenURLLacksDB(t *testing.T) {
	t.Setenv("STAIR_REDIS_PASSWORD", "")
	t.Setenv("STAIR_REDIS_DB", "9")
	t.Setenv("STAIR_REDIS_TLS", "")
	opts := FromEnv("redis://redis:6379")
	if opts.DB != 9 {
		t.Fatalf("DB = %d, want 9", opts.DB)
	}
}

func TestFromEnvPlainAddrPassword(t *testing.T) {
	t.Setenv("STAIR_REDIS_PASSWORD", "secret")
	opts := FromEnv("redis:6379")
	if opts.Password != "secret" {
		t.Fatalf("Password = %q, want secret", opts.Password)
	}
}

func TestParseURLUnsupported(t *testing.T) {
	if _, ok := parseURL("redis:6379"); ok {
		t.Fatal("host:port не должен распознаваться как URL")
	}
	if _, ok := parseURL("postgres://localhost:5432/db"); ok {
		t.Fatal("чуждая схема не должна распознаваться")
	}
	if _, ok := parseURL("redis://"); ok {
		t.Fatal("URL без хоста невалиден")
	}
	if _, ok := parseURL(strings.ToUpper("REDIS://H:1")); ok {
		t.Fatal("схема регистро-чувствительна: uppercase не валидна")
	}
}