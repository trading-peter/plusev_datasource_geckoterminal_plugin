package ratelimits

import (
	"testing"
	"time"

	"github.com/plusev-terminal/go-plugin-common/meta"
)

// Pins the declared GeckoTerminal budget so it cannot silently drift.
// Source (2026-10-31): https://docs.coingecko.com/docs/keyless-public-api
// Keyless public API on api.geckoterminal.com: ~5 calls/min per IP.
func TestGeckoTerminalAPIRules(t *testing.T) {
	rules := GeckoTerminalAPI()
	want := []meta.RateLimitRule{
		{
			ID:       "keyless",
			Label:    "Keyless public API (per IP)",
			Limit:    5,
			WindowMs: time.Minute.Milliseconds(),
		},
	}
	if len(rules) != len(want) {
		t.Fatalf("got %d rules, want %d: %+v", len(rules), len(want), rules)
	}
	for i, r := range rules {
		w := want[i]
		if r.ID != w.ID || r.Label != w.Label || r.Limit != w.Limit ||
			r.WindowMs != w.WindowMs || r.Scope != w.Scope || r.PerKeyEnabled != w.PerKeyEnabled {
			t.Errorf("rule %d = %+v, want %+v", i, r, w)
		}
		if r.Burst != 0 {
			t.Errorf("rule %d Burst = %d, want 0 (defaults to Limit)", i, r.Burst)
		}
		if len(r.Paths) != 0 {
			t.Errorf("rule %d Paths = %v, want none (applies to the whole host)", i, r.Paths)
		}
		if r.CostWeighted {
			t.Errorf("rule %d must not be weighted", i)
		}
	}
}
