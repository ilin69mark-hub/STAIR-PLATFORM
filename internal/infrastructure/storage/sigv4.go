package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// sigV4Signer строит Authorization-заголовок AWS SigV4 (EDR-0026 §3.3)
// на чистой stdlib (crypto/hmac, crypto/sha256). Реализация соответствует
// спецификации: canonical request → string-to-sign → цепочка HMAC-ключей.
type sigV4Signer struct {
	accessKey string
	secretKey string
	region    string
	service   string
}

// newSigV4Signer создаёт подписант для service (обычно "s3").
func newSigV4Signer(accessKey, secretKey, region, service string) *sigV4Signer {
	return &sigV4Signer{accessKey: accessKey, secretKey: secretKey, region: region, service: service}
}

// sign вычисляет Signature для заданного запроса и возвращает полный
// Authorization заголовок. signedHeaders — отсортированные имена заголовков,
// включённых в подпись (строчными буквами).
func (s *sigV4Signer) sign(method, canonicalURI, query string, now time.Time,
	signedHeaders, canonicalHeaders string, payloadHash string) string {
	amzDate := now.UTC().Format("20060102T150405Z")
	dateStamp := now.UTC().Format("20060102")

	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		query,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := strings.Join([]string{dateStamp, s.region, s.service, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hexSHA256([]byte(canonicalRequest)),
	}, "\n")

	signingKey := s.signingKey(dateStamp)
	signature := hmacSHA256Hex(signingKey, []byte(stringToSign))

	return fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKey, scope, signedHeaders, signature)
}

// signingKey строит цепочку HMAC-ключей (EDR-0026 §3.3).
func (s *sigV4Signer) signingKey(dateStamp string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+s.secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(s.region))
	kService := hmacSHA256(kRegion, []byte(s.service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

func hmacSHA256Hex(key, data []byte) string {
	return hex.EncodeToString(hmacSHA256(key, data))
}

func hexSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// uriEncode кодирует путь в SigV4-стиле: каждый сегмент отдельно, пробелы
// как %20, сохраняются "/" разделители.
func uriEncode(p string) string {
	segs := strings.Split(p, "/")
	for i, seg := range segs {
		segs[i] = encodePathSeg(seg)
	}
	return strings.Join(segs, "/")
}

func encodePathSeg(seg string) string {
	esc := url.PathEscape(seg)
	// PathEscape оставляет "/" и "+" нетронутыми внутри сегмента — в SigV4
	// оба кодируются (RFC 3986: "/" → %2F, "+" → %2B).
	esc = strings.ReplaceAll(esc, "/", "%2F")
	esc = strings.ReplaceAll(esc, "+", "%2B")
	return esc
}

// canonicalQuery сортирует query-параметры по имени и кодирует значения.
func canonicalQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	vals, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(uriEncode(k))
		sb.WriteByte('=')
		sb.WriteString(uriEncode(vals[k][0]))
	}
	return sb.String()
}
