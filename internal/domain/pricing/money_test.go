package pricing

import (
	"testing"
)

func TestCurrencyValidate(t *testing.T) {
	if err := CurrencyRUB.Validate(); err != nil {
		t.Fatalf("RUB must validate: %v", err)
	}
	if err := (Currency{}).Validate(); err == nil {
		t.Fatal("empty currency must be rejected")
	}
	if err := (Currency{Code: "RU"}).Validate(); err == nil {
		t.Fatal("2-letter code must be rejected")
	}
	if err := (Currency{Code: "RUBN", Decimals: 2}).Validate(); err == nil {
		t.Fatal("4-letter code must be rejected")
	}
	if err := (Currency{Code: "RUB", Decimals: 5}).Validate(); err == nil {
		t.Fatal("precision above 4 must be rejected")
	}
}

func TestCurrencyFromMajor(t *testing.T) {
	cases := []struct {
		v    float64
		want int64
	}{
		{2.345, 235}, // half-up
		{2.344, 234},
		{0.01, 1},
		{0.005, 1}, // half-up вверх
		{1000000.00, 100000000},
	}
	for _, c := range cases {
		m, err := CurrencyRUB.FromMajor(c.v)
		if err != nil {
			t.Fatalf("FromMajor(%v): %v", c.v, err)
		}
		if m.Minor() != c.want {
			t.Fatalf("FromMajor(%v) = %d, want %d", c.v, m.Minor(), c.want)
		}
	}
	if _, err := CurrencyRUB.FromMajor(-1); err == nil {
		t.Fatal("negative amount must be rejected")
	}
	if _, err := CurrencyRUB.FromMajor(1e300); err == nil {
		t.Fatal("overflow must be rejected")
	}
}

func TestMoneyMajor(t *testing.T) {
	m := NewMoney(12345)
	if m.Major(CurrencyRUB) != 123.45 {
		t.Fatalf("Major = %v, want 123.45", m.Major(CurrencyRUB))
	}
}

func TestMoneyArithmetic(t *testing.T) {
	a := NewMoney(1000)
	b := NewMoney(250)
	if got := a.Add(b).Minor(); got != 1250 {
		t.Fatalf("Add = %d, want 1250", got)
	}
	if got := a.Sub(b).Minor(); got != 750 {
		t.Fatalf("Sub = %d, want 750", got)
	}
}

func TestMoneyMulRate(t *testing.T) {
	cases := []struct {
		money int64
		rate  float64
		want  int64
	}{
		{1000, 20, 200},   // 20%
		{1000, 5, 50},     // 5%
		{333, 33.33, 111}, // 333×33330/100000=110.989 → 111
		{1000, 100, 1000}, // 100%
		{333, 0.001, 0},   // 333×1/100000 → 0
		{1000, 20.5, 205}, // 20.5%
	}
	for _, c := range cases {
		r, err := NewRate(c.rate)
		if err != nil {
			t.Fatal(err)
		}
		if got := NewMoney(c.money).MulRate(r).Minor(); got != c.want {
			t.Fatalf("%d×%v%% = %d, want %d", c.money, c.rate, got, c.want)
		}
	}
}

func TestNewRate(t *testing.T) {
	r, err := NewRate(10.5)
	if err != nil {
		t.Fatal(err)
	}
	if r != 10500 {
		t.Fatalf("10.5%% = %d, want 10500", r)
	}
	if _, err := NewRate(-1); err == nil {
		t.Fatal("negative rate must be rejected")
	}
	if _, err := NewRate(1e300); err == nil {
		t.Fatal("overflow must be rejected")
	}
}

func TestMustRate(t *testing.T) {
	if got := MustRate(20); got != 20000 {
		t.Fatalf("MustRate(20) = %d, want 20000", got)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("MustRate(-1) must panic")
		}
	}()
	MustRate(-1)
}

func TestRatePercent(t *testing.T) {
	if got := Rate(10500).Percent(); got != 10.5 {
		t.Fatalf("Percent = %v, want 10.5", got)
	}
}
