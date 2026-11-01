package main

import (
	"sort"
	"strings"
	"time"

	ex "github.com/plusev-terminal/go-plugin-common/v2/datasrc/exchange"
	"github.com/plusev-terminal/go-plugin-common/v2/logging"
	m "github.com/plusev-terminal/go-plugin-common/v2/meta"
	"github.com/plusev-terminal/go-plugin-common/v2/plugin"
	"github.com/plusev-terminal/go-plugin-common/v2/requester"

	"github.com/trading-peter/plusev_datasource_geckoterminal_plugin/geckoterminal"
	"github.com/trading-peter/plusev_datasource_geckoterminal_plugin/ratelimits"
)

// ============================================================================
// Main - Register the plugin
// ============================================================================

func init() {
	p := &GeckoTerminalPlugin{}
	plugin.RegisterPlugin(p)
}

func main() {}

// ============================================================================
// Plugin Implementation
// ============================================================================

var log = logging.NewLogger("geckoterminal")

type GeckoTerminalPlugin struct {
	config *plugin.ConfigStore
	client *geckoterminal.Client
}

func (p *GeckoTerminalPlugin) GetMeta() m.Meta {
	return m.Meta{
		PluginID:    "geckoterminal-datasource",
		Name:        "GeckoTerminal DEX Prices",
		AppID:       "datasrc",
		Category:    "dex",
		Description: "Historical OHLCV price data from DEX pools via GeckoTerminal. Supports 6M+ tokens across 1,500+ DEXes including Raydium, Uniswap, PancakeSwap and more. Ideal for meme coin and exotic cross-pair cost basis pricing.",
		Author:      "trading_peter",
		Version: "v1.1.0",
		Repository:  "https://github.com/trading-peter/plusev_datasource_geckoterminal_plugin",
		Tags:        []string{"geckoterminal", "dex", "ohlcv", "meme", "solana", "ethereum", "defi"},
		Contacts: []m.AuthorContact{
			{Kind: "x.com", Value: "https://x.com/trading_peter"},
		},
		Resources: m.ResourceAccess{
			AllowedNetworkTargets: []m.NetworkTargetRule{
				{
					Pattern: "https://api.geckoterminal.com/*",
					// Free-tier budgets with sources; see package ratelimits.
					Limits: ratelimits.GeckoTerminalAPI(),
				},
			},
		},
		Features: []string{},
	}
}

func (p *GeckoTerminalPlugin) GetConfigFields() []plugin.ConfigField {
	if p.client == nil {
		p.client = geckoterminal.NewClient(requester.NewRequester())
	}

	return p.client.GetConfigFields()
}

func (p *GeckoTerminalPlugin) OnInit(config *plugin.ConfigStore) error {
	p.config = config
	p.client = geckoterminal.NewClient(requester.NewRequester())

	if network := config.GetString("network"); network != "" {
		p.client.SetNetwork(network)
	}

	return nil
}

func (p *GeckoTerminalPlugin) OnShutdown() error {
	return nil
}

func (p *GeckoTerminalPlugin) GetRateLimits() []plugin.RateLimit {
	// Opt out of the legacy per-command limiter: api.geckoterminal.com is paced
	// by the manifest network-target limits declared in GetMeta, and an empty
	// (but non-nil) return keeps the export while avoiding double throttling.
	return []plugin.RateLimit{}
}

func (p *GeckoTerminalPlugin) RegisterCommands(router *plugin.CommandRouter) {
	router.Register(ex.CMD_GET_MARKET, p.handleGetMarket)
	router.Register(ex.CMD_GET_TIMEFRAMES, p.handleGetTimeframes)
	router.Register(ex.CMD_GET_OHLCV, p.handleGetOHLCV)
}

// ============================================================================
// Command Handlers
// ============================================================================

func (p *GeckoTerminalPlugin) handleGetMarket(params map[string]any) plugin.Response {
	mp := ex.GetMarketParamsFromMap(params)
	if err := mp.Validate(); err != nil {
		return plugin.ErrorResponseMsg(err.Error())
	}

	parts := strings.SplitN(mp.Symbol, "/", 2)
	quote := ""
	if len(parts) == 2 {
		quote = parts[1]
	}

	// If a token mint address is provided, find pools containing that token.
	if mp.Address != "" {
		market, err := p.client.GetPoolForToken(mp.Address, quote)
		if err != nil {
			return plugin.ErrorResponse(err)
		}
		return plugin.SuccessResponse(market, time.Hour)
	}

	// Otherwise search by symbol (e.g. "SOL/USDC").
	if len(parts) != 2 {
		return plugin.ErrorResponseMsg("symbol must be in BASE/QUOTE format")
	}

	market, err := p.client.SearchPool(parts[0], parts[1])
	if err != nil {
		return plugin.ErrorResponse(err)
	}

	return plugin.SuccessResponse(market, time.Hour)
}

func (p *GeckoTerminalPlugin) handleGetTimeframes(params map[string]any) plugin.Response {
	return plugin.SuccessResponse(p.client.GetTimeframes(), time.Hour*24)
}

func (p *GeckoTerminalPlugin) handleGetOHLCV(params map[string]any) plugin.Response {
	ohlcvParams := ex.GetOHLCVParamsFromMap(params)

	if ohlcvParams.Market.Symbol == "" {
		return plugin.ErrorResponseMsg("market.symbol is required")
	}

	// Pool address is stored in Market.Address by getMarkets.
	poolAddress := ohlcvParams.Market.Address
	if poolAddress == "" {
		return plugin.ErrorResponseMsg("market.address (pool address) is required — use a market returned by getMarket")
	}

	if ohlcvParams.Timeframe == "" {
		return plugin.ErrorResponseMsg("timeframe is required")
	}

	var startUnix, endUnix int64
	if ohlcvParams.StartTime != nil {
		startUnix = ohlcvParams.StartTime.Unix()
	}
	if ohlcvParams.EndTime != nil {
		endUnix = ohlcvParams.EndTime.Unix()
	} else {
		endUnix = time.Now().Unix()
	}

	records, err := p.client.GetOHLCV(poolAddress, ohlcvParams.Timeframe, startUnix, endUnix, ohlcvParams.Limit)
	if err != nil {
		log.ErrorWithData("handleGetOHLCV error", map[string]any{
			"error":     err.Error(),
			"pool":      poolAddress,
			"timeframe": ohlcvParams.Timeframe,
		})
		return plugin.ErrorResponse(err)
	}

	// API returns newest-first; sort ascending by default.
	sort.Slice(records, func(i, j int) bool {
		return records[i].OpenTime < records[j].OpenTime
	})

	if ohlcvParams.Sort == "desc" {
		for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
			records[i], records[j] = records[j], records[i]
		}
	}

	if ohlcvParams.Limit > 0 && len(records) > ohlcvParams.Limit {
		records = records[:ohlcvParams.Limit]
	}

	if ohlcvParams.CacheForSeconds > 0 {
		cacheDuration := time.Duration(ohlcvParams.CacheForSeconds) * time.Second
		return plugin.SuccessResponse(records, cacheDuration)
	}

	return plugin.SuccessResponse(records)
}
