package storage

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// S3Store — ObjectStore поверх S3-совместимого хранилища (EDR-0026 §3.3):
// PUT/GET/DELETE объектов bucket/key с подписью AWS SigV4. Поддержан
// path-style (bucket в пути) для MinIO и virtual-hosted по endpoint.
type S3Store struct {
	endpoint  string // http(s)://host[:port]
	bucket    string
	pathStyle bool
	signer    *sigV4Signer
	client    *http.Client
}

// NewS3Store создаёт S3-бэкенд (EDR-0026 §3.3).
func NewS3Store(o Options) (*S3Store, error) {
	if o.Endpoint == "" {
		return nil, fmt.Errorf("%w: s3 endpoint required", ErrInvalid)
	}
	if o.Bucket == "" {
		return nil, fmt.Errorf("%w: s3 bucket required", ErrInvalid)
	}
	if o.AccessKey == "" || o.SecretKey == "" {
		return nil, fmt.Errorf("%w: s3 credentials required", ErrInvalid)
	}
	region := o.Region
	if region == "" {
		region = "us-east-1"
	}
	return &S3Store{
		endpoint:  strings.TrimSuffix(o.Endpoint, "/"),
		bucket:    o.Bucket,
		pathStyle: o.PathStyle,
		signer:    newSigV4Signer(o.AccessKey, o.SecretKey, region, "s3"),
		client:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// objectURL строит URL объекта с учётом path-style.
func (s *S3Store) objectURL(key string) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	if s.pathStyle {
		return s.endpoint + "/" + s.bucket + "/" + key, nil
	}
	return s.endpoint + "/" + key, nil
}

// s3Error — тело ошибки S3 (Code/Message).
type s3Error struct {
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// do выполняет подписанный запрос и обрабатывает ответ.
func (s *S3Store) do(ctx context.Context, method, target string, key string, body []byte, contentType string) error {
	payloadHash := hexSHA256(body)
	now := time.Now()
	amzDate := now.UTC().Format("20060102T150405Z")

	u, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("storage: parse url: %w", err)
	}

	// Host + amz headers включаются в подпись.
	headers := map[string]string{
		"host":                 u.Host,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           amzDate,
	}
	if contentType != "" {
		headers["content-type"] = contentType
	}

	names := make([]string, 0, len(headers))
	for n := range headers {
		names = append(names, n)
	}
	sort.Strings(names)
	var canonicalHeaders, signedHeaders strings.Builder
	for i, n := range names {
		if i > 0 {
			canonicalHeaders.WriteByte('\n')
		}
		canonicalHeaders.WriteString(n)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.TrimSpace(headers[n]))
		signedHeaders.WriteString(n)
		if i < len(names)-1 {
			signedHeaders.WriteByte(';')
		}
	}

	auth := s.signer.sign(method, uriEncode(u.Path), u.RawQuery, now,
		signedHeaders.String(), canonicalHeaders.String()+"\n", payloadHash)

	req, err := http.NewRequestWithContext(ctx, method, target, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("storage: new request: %w", err)
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Header.Set("X-Amz-Date", amzDate)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("storage: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var se s3Error
	_ = xml.Unmarshal(raw, &se)
	code := se.Code
	if code == "" {
		code = resp.Status
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s", ErrNotFound, code)
	}
	return fmt.Errorf("storage: s3 %s %s: %s", method, target, code)
}

// Put сохраняет объект (EDR-0026 §3.3).
func (s *S3Store) Put(ctx context.Context, key string, data []byte, contentType string) error {
	u, err := s.objectURL(key)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodPut, u, key, data, contentType)
}

// Get возвращает данные объекта (EDR-0026 §3.3).
func (s *S3Store) Get(ctx context.Context, key string) ([]byte, error) {
	u, err := s.objectURL(key)
	if err != nil {
		return nil, err
	}
	payloadHash := hexSHA256(nil)
	now := time.Now()
	amzDate := now.UTC().Format("20060102T150405Z")

	parsed, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("storage: parse url: %w", err)
	}
	headers := map[string]string{
		"host":                 parsed.Host,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           amzDate,
	}
	names := make([]string, 0, len(headers))
	for n := range headers {
		names = append(names, n)
	}
	sort.Strings(names)
	var canonicalHeaders, signedHeaders strings.Builder
	for i, n := range names {
		if i > 0 {
			canonicalHeaders.WriteByte('\n')
		}
		canonicalHeaders.WriteString(n)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.TrimSpace(headers[n]))
		signedHeaders.WriteString(n)
		if i < len(names)-1 {
			signedHeaders.WriteByte(';')
		}
	}
	auth := s.signer.sign(http.MethodGet, uriEncode(parsed.Path), parsed.RawQuery, now,
		signedHeaders.String(), canonicalHeaders.String()+"\n", payloadHash)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("storage: new request: %w", err)
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Header.Set("X-Amz-Date", amzDate)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("storage: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var se s3Error
		_ = xml.Unmarshal(raw, &se)
		return nil, fmt.Errorf("storage: s3 GET %s: %s", u, se.Code)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("storage: read body: %w", err)
	}
	return data, nil
}

// Delete удаляет объект (EDR-0026 §3.3).
func (s *S3Store) Delete(ctx context.Context, key string) error {
	u, err := s.objectURL(key)
	if err != nil {
		return err
	}
	return s.do(ctx, http.MethodDelete, u, key, nil, "")
}
