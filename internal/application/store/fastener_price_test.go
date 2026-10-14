package store

import (
	"context"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// SetFastenerPrice защищает витрину от «бесплатного крепежа».
//
// Ошибка здесь не в цене, а в её отсутствии: опечатка в имени поля дала бы
// нулевую цену с ответом 200, и крепёж ушёл бы в смету бесплатно. Поэтому
// неположительная цена отвергается, а не принимается как «бесплатно».

func TestSetFastenerPriceRejectsBadInput(t *testing.T) {
	svc := NewService(newFakeRepo())
	ctx := context.Background()
	code := dommfg.ScrewTreadRiserCode

	// Пустой тенант — такой записи не существует.
	if err := svc.SetFastenerPrice(ctx, "", code, 100, "u-1"); err == nil {
		t.Error("пустой тенант принят")
	}
	// Код не из каталога: иначе в смету попала бы позиция без крепежа.
	if err := svc.SetFastenerPrice(ctx, "t-1", "SCREW-НЕТ-ТАКОГО", 100, "u-1"); err == nil {
		t.Error("неизвестный код крепежа принят")
	}
	// Неположительные цены — ровно тот случай из комментария в коде.
	for _, price := range []int64{0, -15} {
		if err := svc.SetFastenerPrice(ctx, "t-1", code, price, "u-1"); err == nil {
			t.Errorf("цена %d принята как корректная", price)
		}
	}
}

func TestSetFastenerPriceAcceptsValid(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	code := dommfg.ScrewTreadRiserCode

	if err := svc.SetFastenerPrice(context.Background(), "t-1", code, 150, "u-1"); err != nil {
		t.Fatalf("корректная цена отвергнута: %v", err)
	}
	got, err := repo.ListFastenerPrices(context.Background(), "t-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if p.Code == code && p.PricePerPcRub != 150 {
			t.Errorf("цена сохранена как %d, ждали 150", p.PricePerPcRub)
		}
	}
}

func TestDeleteFastenerPriceReturnsToBuiltInRate(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()
	code := dommfg.ScrewTreadRiserCode

	if err := svc.SetFastenerPrice(ctx, "t-1", code, 150, "u-1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteFastenerPrice(ctx, "t-1", code); err != nil {
		t.Fatalf("удаление цены вернуло ошибку: %v", err)
	}
	// После удаления цена не должна остаться пользовательской.
	got, _ := repo.ListFastenerPrices(ctx, "t-1")
	for _, p := range got {
		if p.Code == code && p.PricePerPcRub == 150 {
			t.Error("после удаления осталась пользовательская цена")
		}
	}
	// Пустой тенант отвергается.
	if err := svc.DeleteFastenerPrice(ctx, "", code); err == nil {
		t.Error("удаление для пустого тенанта принято")
	}
}
