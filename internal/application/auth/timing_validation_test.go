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

// TestLoginTimingEqualized — S-113: после фикса dummy-bcrypt неизвестный
// email проходит bcrypt-сравнение, и время ответа не выдаёт существование
// аккаунта. Нижняя граница (≥10 мс) доказывает, что bcrypt выполнен;
// верхняя (≤ 5×known + 20 мс) — что ветки сравнялись (щедрый допуск на
// шум CI/race).
func TestLoginTimingEqualized(t *testing.T) {
	repo := newFakeRepo()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateUser(context.Background(), &User{
		Email:        "known@example.com",
		PasswordHash: string(hash),
		Status:       StatusActive,
	}); err != nil {
		t.Fatal(err)
	}
	svc := NewService(repo, time.Hour)
	ctx := context.Background()

	// Прогрев: ленивая генерация dummy-хеша + JIT.
	_, _, _ = svc.Login(ctx, "unknown@example.com", "x")
	_, _, _ = svc.Login(ctx, "known@example.com", "wrong")

	measure := func(email, password string) time.Duration {
		start := time.Now()
		_, _, _ = svc.Login(ctx, email, password)
		return time.Since(start)
	}

	const samples = 5
	var unknown, known time.Duration
	for i := 0; i < samples; i++ {
		unknown += measure("unknown@example.com", "x")
		known += measure("known@example.com", "wrong")
	}
	unknown /= samples
	known /= samples

	if unknown < 10*time.Millisecond {
		t.Errorf("unknown-email login too fast (%v): dummy bcrypt compare not executed", unknown)
	}
	if unknown > known*5+20*time.Millisecond {
		t.Errorf("unknown (%v) still much slower than known (%v) — timing leak remains", unknown, known)
	}
}
