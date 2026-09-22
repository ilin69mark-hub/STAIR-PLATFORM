package auth

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// S-113 / LOGIN-TIMING-ENUMERATION: замер разницы латентности Login для
// известного и неизвестного email.
//
// Гипотеза аудита: service.go:191 возвращает ErrInvalidCreds мгновенно для
// неизвестного email, а для известного (service.go:199) выполняется
// bcrypt.CompareHashAndPassword (~50-100 мс) — измеримая разница позволяет
// перечислять пользователей по времени ответа.
//
// Запуск: go test -bench=BenchmarkLoginTiming -benchtime=20x -run=^$ ./internal/application/auth/
func BenchmarkLoginTimingKnownEmail(b *testing.B) {
	repo := newFakeRepo()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	if err != nil {
		b.Fatal(err)
	}
	if err := repo.CreateUser(context.Background(), &User{
		Email:        "known@example.com",
		PasswordHash: string(hash),
		Status:       StatusActive,
	}); err != nil {
		b.Fatal(err)
	}
	svc := NewService(repo, time.Hour)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Неверный пароль: путь проходит bcrypt-сравнение и возвращает
		// ErrInvalidCreds (service.go:199-201) — без создания сессии.
		if _, _, err := svc.Login(ctx, "known@example.com", "wrong-password"); err != ErrInvalidCreds {
			b.Fatalf("expected ErrInvalidCreds, got %v", err)
		}
	}
}

func BenchmarkLoginTimingUnknownEmail(b *testing.B) {
	repo := newFakeRepo()
	svc := NewService(repo, time.Hour)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := svc.Login(ctx, "unknown@example.com", "whatever"); err != ErrInvalidCreds {
			b.Fatalf("expected ErrInvalidCreds, got %v", err)
		}
	}
}
