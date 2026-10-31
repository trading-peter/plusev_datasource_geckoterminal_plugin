// Package ratelimits declares the GeckoTerminal request budgets this plugin
// paces itself against. The number is the keyless public tier (this plugin
// sends no API key); users with a free CoinGecko demo key or a paid plan
// raise it via Terminal's Rate Limits UI override.
//
// Sources (retrieved 2026-10-31):
//   - https://docs.coingecko.com/docs/keyless-public-api
//     ("GeckoTerminal endpoints: ~5 calls/min (dynamic, varies by server
//     load)"; keyless requests share an IP-based rate pool on
//     api.geckoterminal.com — the base URL this plugin calls)
//   - https://support.coingecko.com/articles/6867086195-does-geckoterminal-have-an-api
//     (30 calls/min applies to the free demo API key, which this plugin
//     does not send; docs are ambiguous between the two figures, so the
//     conservative keyless number is declared)
package ratelimits

import (
	"time"

	"github.com/plusev-terminal/go-plugin-common/meta"
)

// GeckoTerminalAPI returns the manifest rate-limit rules for
// api.geckoterminal.com. The keyless pool is IP-based, so this is a single
// per-host (per-IP) bucket shared by all endpoints.
func GeckoTerminalAPI() []meta.RateLimitRule {
	return []meta.RateLimitRule{
		// Keyless public API: ~5 calls/min per IP.
		// https://docs.coingecko.com/docs/keyless-public-api
		meta.NewRateLimitRule("keyless", 5, time.Minute).
			WithLabel("Keyless public API (per IP)"),
	}
}
