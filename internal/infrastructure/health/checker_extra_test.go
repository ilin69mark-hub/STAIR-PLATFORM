package health

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestItoa(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{{0, "0"}, {5, "5"}, {42, "42"}, {12345, "12345"}}
	for _, c := range cases {
		if got := itoa(c.n); got != c.want {
			t.Fatalf("itoa(%d)=%q want %q", c.n, got, c.want)
		}
	}
}

func TestFormatPoolStats(t *testing.T) {
	// Just ensure it returns non-empty string with keys
	// Use real pgxpool.Stat from nil pool? Create via parsing config without connect
	cfg, _ := pgxpool.ParseConfig("postgres://user:pass@localhost:5432/db")
	pool, _ := pgxpool.NewWithConfig(context.Background(), cfg)
	if pool == nil {
		t.Skip("pool nil")
	}
	defer pool.Close()
	s := formatPoolStats(pool.Stat())
	if s == "" || len(s) < 10 {
		t.Fatalf("formatPoolStats empty: %q", s)
	}
	for _, want := range []string{"totalConns=", "idleConns=", "acquiredConns="} {
		found := false
		for i := 0; i < len(s)-len(want); i++ {
			if s[i:i+len(want)] == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("want %q in %q", want, s)
		}
	}
}

func TestReadyNoProviders(t *testing.T) {
	c := &Checker{}
	ok, m := c.Ready(context.Background())
	if !ok || len(m) != 0 {
		t.Fatalf("want ok empty, got %v %v", ok, m)
	}
	deep := c.CheckDeep(context.Background())
	if deep.Status != "ok" || len(deep.Checks) != 0 {
		t.Fatalf("deep want ok empty, got %+v", deep)
	}
}

func TestCheckDeepTimeoutDefault(t *testing.T) {
	c := &Checker{Timeout: 0}
	deep := c.CheckDeep(context.Background())
	if deep.Status != "ok" {
		t.Fatalf("want ok, got %v", deep.Status)
	}
	_ = time.Now()
}
