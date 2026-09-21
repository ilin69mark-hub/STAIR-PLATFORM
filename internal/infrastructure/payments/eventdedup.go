package payments

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// EventDeduper — защита от повторной обработки webhook-событий (replay /
// повторная доставка провайдера, P1-1). Реализация на Redis атомарна
// (SETNX), поэтому два одновременно пришедших дубликата не обработаются
// дважды.
type EventDeduper interface {
	// CheckAndMark помечает eventID обработанным. Возвращает true, если событие
	// новое (и помечено); false, если уже встречалось.
	CheckAndMark(ctx context.Context, provider, eventID string) (bool, error)
	// Clear снимает метку: вызывается, если обработка события завершилась
	// ошибкой, чтобы повторная доставка провайдера применилась.
	Clear(ctx context.Context, provider, eventID string) error
}

// eventDedupTTL — срок жизни метки. Stripe повторяет доставку в течение
// нескольких дней только при ошибках; успешные события не меняются, ключ
// с TTL сам истекает без гонок.
const eventDedupTTL = 7 * 24 * time.Hour

const eventDedupKeyPrefix = "stair:webhook:"

func eventDedupKey(provider, eventID string) string {
	return eventDedupKeyPrefix + provider + ":" + eventID
}

// RedisEventDeduper — дедупликация через SETNX в Redis.
type RedisEventDeduper struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisEventDeduper создаёт Redis-дедупера. client обязателен.
func NewRedisEventDeduper(client *redis.Client) *RedisEventDeduper {
	return &RedisEventDeduper{client: client, ttl: eventDedupTTL}
}

// CheckAndMark атомарно помечает событие: SETNX по ключу провайдер:eventID.
// Новое событие → true; повтор → false.
func (d *RedisEventDeduper) CheckAndMark(ctx context.Context, provider, eventID string) (bool, error) {
	return d.client.SetNX(ctx, eventDedupKey(provider, eventID), "1", d.ttl).Result()
}

// Clear удаляет метку обработанного события.
func (d *RedisEventDeduper) Clear(ctx context.Context, provider, eventID string) error {
	return d.client.Del(ctx, eventDedupKey(provider, eventID)).Err()
}
