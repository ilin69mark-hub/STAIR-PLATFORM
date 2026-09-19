// Package redisconf собирает параметры go-redis из строки адреса и переменных
// окружения (P0-2). Поддерживаются две формы STAIR_REDIS_ADDR:
//
//	"host:port"                       — чистый адрес (дефолт для go-redis)
//	"redis://[:password@]host:port[/db]"  — URL-схема
//	"rediss://[:password@]host:port[/db]" — URL-схема с TLS
//
// Параметры окружения (применяются, если не заданы в URL):
//   - STAIR_REDIS_PASSWORD — пароль (иначе берётся userinfo из URL)
//   - STAIR_REDIS_DB       — номер логической БД (иначе path из URL)
//   - STAIR_REDIS_TLS      — "true" включает TLS (rediss:// тоже включает)
//
// Единая точка сборки Options для API (лимитеры/очередь/readiness) и воркера.
package redisconf

import (
	"crypto/tls"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

// FromEnv строит *redis.Options по адресу и env-переменным STAIR_REDIS_*.
// При непарсируемом URL адрес передаётся в go-redis как есть (host:port) —
// этот формат не пострадает, схема лишь добавляет приоритетные параметры.
func FromEnv(addr string) *redis.Options {
	opts := &redis.Options{Addr: strings.TrimSpace(addr)}

	parsed, ok := parseURL(addr)
	if ok {
		opts.Addr = parsed.Host
		opts.Password = parsed.Password
		if parsed.HasDB {
			opts.DB = parsed.DB
		}
		if parsed.TLS {
			opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
	}

	// Env-параметры дозаполняют то, что не задано URL-формой.
	if opts.Password == "" {
		if v := os.Getenv("STAIR_REDIS_PASSWORD"); v != "" {
			opts.Password = v
		}
	}
	if !parsed.HasDB {
		if v := os.Getenv("STAIR_REDIS_DB"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				opts.DB = n
			}
		}
	}
	if opts.TLSConfig == nil {
		if s := strings.ToLower(os.Getenv("STAIR_REDIS_TLS")); s == "true" || s == "1" {
			opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
	}

	return opts
}

type urlInfo struct {
	Host     string
	Password string
	DB       int
	HasDB    bool
	TLS      bool
}

// parseURL разбирает redis:// / rediss:// URL и возвращает false для
// остальных форм (host:port, unix, окружения без адреса).
func parseURL(addr string) (urlInfo, bool) {
	if !strings.Contains(addr, "://") {
		return urlInfo{}, false
	}
	scheme, _, _ := strings.Cut(addr, "://")
	if scheme != "redis" && scheme != "rediss" {
		return urlInfo{}, false
	}
	u, err := url.Parse(addr)
	if err != nil || u.Host == "" {
		return urlInfo{}, false
	}
	info := urlInfo{
		Host: u.Host,
		TLS:  scheme == "rediss",
	}
	if u.User != nil {
		if pwd, ok := u.User.Password(); ok && pwd != "" {
			info.Password = pwd
		}
	}
	if n, err := strconv.Atoi(strings.TrimPrefix(u.Path, "/")); err == nil && n >= 0 {
		info.DB = n
		info.HasDB = true
	}
	return info, true
}