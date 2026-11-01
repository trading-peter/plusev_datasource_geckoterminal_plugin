package geckoterminal

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/plusev-terminal/go-plugin-common/v2/logging"
	"github.com/plusev-terminal/go-plugin-common/v2/plugin"
	rt "github.com/plusev-terminal/go-plugin-common/v2/requester/types"
	"github.com/plusev-terminal/go-plugin-common/v2/trading"
)

const baseURL = "https://api.geckoterminal.com/api/v2"

var log = logging.NewLogger("geckoterminal")

// Client handles all HTTP communication with the GeckoTerminal API.
type Client struct {
	requester rt.RequestDoer
	network   string
}

// NewClient creates a new GeckoTerminal API client.
func NewClient(req rt.RequestDoer) *Client {
	return &Client{
		requester: req,
		network:   "solana",
	}
}

// SetNetwork sets the blockchain network for API calls.
func (c *Client) SetNetwork(network string) {
	c.network = network
}

// GetConfigFields returns the configuration fields for this plugin.
func (c *Client) GetConfigFields() []plugin.ConfigField {
	return []plugin.ConfigField{
		{
			Name:          "network",
			Label:         "Blockchain Network",
			Type:          "select",
			Required:      true,
			Description:   "Blockchain network to query DEX pools on.",
			DefaultOption: "solana",
			Options: []plugin.ConfigFieldSelectOption{
				{Value: "solana", Label: "Solana"},
				{Value: "eth", Label: "Ethereum"},
				{Value: "base", Label: "Base"},
				{Value: "arbitrum", Label: "Arbitrum"},
				{Value: "bsc", Label: "BNB Smart Chain"},
				{Value: "polygon_pos", Label: "Polygon"},
				{Value: "avalanche", Label: "Avalanche"},
				{Value: "optimism", Label: "Optimism"},
				{Value: "fantom", Label: "Fantom"},
				{Value: "celo", Label: "Celo"},
				{Value: "sui-network", Label: "Sui"},
				{Value: "aptos", Label: "Aptos"},
				{Value: "ton", Label: "TON"},
			},
		},
	}
}

// ============================================================================
// Pool Discovery
// ============================================================================

// resolveTokenSymbols extracts base and quote symbols from pool data and included tokens.
func (c *Client) resolveTokenSymbols(pool PoolData, tokens map[string]IncludedToken) (base, quote string) {
	baseID := pool.Relationships.BaseToken.Data.ID
	quoteID := pool.Relationships.QuoteToken.Data.ID

	if t, ok := tokens[baseID]; ok {
		base = t.Attributes.Symbol
	}
	if t, ok := tokens[quoteID]; ok {
		quote = t.Attributes.Symbol
	}

	// If the "included" data didn't have the symbol, try to extract from the pool name.
	if (base == "" || quote == "") && pool.Attributes.Name != "" {
		parts := strings.SplitN(pool.Attributes.Name, " / ", 2)
		if len(parts) == 2 {
			if base == "" {
				base = strings.TrimSpace(parts[0])
			}
			if quote == "" {
				quote = strings.TrimSpace(parts[1])
			}
		}
	}

	return base, quote
}

// GetPoolByAddress fetches a single pool by its on-chain address and returns it
// as a trading.Market.
func (c *Client) GetPoolByAddress(poolAddress string) (trading.Market, error) {
	url := fmt.Sprintf("%s/networks/%s/pools/%s?include=base_token,quote_token",
		baseURL, c.network, poolAddress)

	var resp SinglePoolResponse
	_, err := c.requester.Send(&rt.Request{
		URL:    url,
		Method: "GET",
		Headers: map[string]string{
			"Accept": "application/json",
		},
	}, &resp)
	if err != nil {
		return trading.Market{}, fmt.Errorf("failed to fetch pool %s: %w", poolAddress, err)
	}

	tokenMap := make(map[string]IncludedToken, len(resp.Included))
	for _, t := range resp.Included {
		tokenMap[t.ID] = t
	}

	base, quote := c.resolveTokenSymbols(resp.Data, tokenMap)

	return trading.Market{
		Symbol:    base + "/" + quote,
		Label:     base + "/" + quote,
		Address:   resp.Data.Attributes.Address,
		Base:      base,
		Quote:     quote,
		AssetType: trading.AssetTypeSpot,
	}, nil
}

// GetPoolForToken finds the top-liquidity pool for a token mint address.
// If quote is non-empty, only pools where the quote token symbol matches are considered.
func (c *Client) GetPoolForToken(mintAddress, quote string) (trading.Market, error) {
	url := fmt.Sprintf("%s/networks/%s/tokens/%s/pools?include=base_token,quote_token&page=1",
		baseURL, c.network, mintAddress)

	var resp SearchPoolsResponse
	_, err := c.requester.Send(&rt.Request{
		URL:    url,
		Method: "GET",
		Headers: map[string]string{
			"Accept": "application/json",
		},
	}, &resp)
	if err != nil {
		return trading.Market{}, fmt.Errorf("failed to fetch pools for token %s: %w", mintAddress, err)
	}

	tokenMap := make(map[string]IncludedToken, len(resp.Included))
	for _, t := range resp.Included {
		tokenMap[t.ID] = t
	}

	upQuote := strings.ToUpper(quote)

	// Pools are returned sorted by liquidity (highest first).
	// Pick the first one that matches the quote filter (if provided).
	for _, pool := range resp.Data {
		b, q := c.resolveTokenSymbols(pool, tokenMap)

		if upQuote != "" && strings.ToUpper(q) != upQuote {
			continue
		}

		return trading.Market{
			Symbol:    b + "/" + q,
			Label:     b + "/" + q,
			Address:   pool.Attributes.Address,
			Base:      b,
			Quote:     q,
			AssetType: trading.AssetTypeSpot,
		}, nil
	}

	if quote != "" {
		return trading.Market{}, fmt.Errorf("no pool found for token %s with quote %s on %s", mintAddress, quote, c.network)
	}

	return trading.Market{}, fmt.Errorf("no pool found for token %s on %s", mintAddress, c.network)
}

// SearchPool searches for a pool matching the given base/quote pair and returns
// it as a trading.Market. It uses the /search/pools endpoint, filtering results
// to the configured network and picking the highest-liquidity match.
func (c *Client) SearchPool(base, quote string) (trading.Market, error) {
	query := base + "/" + quote
	url := fmt.Sprintf("%s/search/pools?query=%s&network=%s&page=1&include=base_token,quote_token",
		baseURL, query, c.network)

	var resp SearchPoolsResponse
	_, err := c.requester.Send(&rt.Request{
		URL:    url,
		Method: "GET",
		Headers: map[string]string{
			"Accept": "application/json",
		},
	}, &resp)
	if err != nil {
		return trading.Market{}, fmt.Errorf("pool search failed: %w", err)
	}

	tokenMap := make(map[string]IncludedToken, len(resp.Included))
	for _, t := range resp.Included {
		tokenMap[t.ID] = t
	}

	upBase := strings.ToUpper(base)
	upQuote := strings.ToUpper(quote)

	for _, pool := range resp.Data {
		b, q := c.resolveTokenSymbols(pool, tokenMap)
		if strings.ToUpper(b) != upBase || strings.ToUpper(q) != upQuote {
			continue
		}

		return trading.Market{
			Symbol:    b + "/" + q,
			Label:     b + "/" + q,
			Address:   pool.Attributes.Address,
			Base:      b,
			Quote:     q,
			AssetType: trading.AssetTypeSpot,
		}, nil
	}

	return trading.Market{}, fmt.Errorf("no pool found for %s/%s on %s", base, quote, c.network)
}

// ============================================================================
// OHLCV Data
// ============================================================================

// timeframeMap maps our standard timeframe strings to GeckoTerminal's expected values.
var timeframeMap = map[string]string{
	"1m":  "minute",
	"5m":  "minute",
	"15m": "minute",
	"1h":  "hour",
	"4h":  "hour",
	"6h":  "hour",
	"12h": "hour",
	"1D":  "day",
}

// aggregateMap maps our timeframe strings to the aggregate parameter.
var aggregateMap = map[string]int{
	"1m":  1,
	"5m":  5,
	"15m": 15,
	"1h":  1,
	"4h":  4,
	"6h":  6,
	"12h": 12,
	"1D":  1,
}

// GetOHLCV fetches historical OHLCV candle data for a pool.
// poolAddress is the on-chain pool address (stored in Market.Address).
// timeframe is one of: "1m", "5m", "15m", "1h", "4h", "6h", "12h", "1D".
func (c *Client) GetOHLCV(poolAddress, timeframe string, startUnix, endUnix int64, limit int) ([]trading.OHLCVRecord, error) {
	tf, ok := timeframeMap[timeframe]
	if !ok {
		return nil, fmt.Errorf("unsupported timeframe: %s", timeframe)
	}

	aggregate := aggregateMap[timeframe]

	var allRecords []trading.OHLCVRecord
	beforeTs := endUnix
	maxPerRequest := 1000
	maxPages := 10

	if limit > 0 && limit < maxPerRequest {
		maxPerRequest = limit
	}

	for page := 0; page < maxPages; page++ {
		url := fmt.Sprintf("%s/networks/%s/pools/%s/ohlcv/%s?aggregate=%d&before_timestamp=%d&limit=%d&currency=usd",
			baseURL, c.network, poolAddress, tf, aggregate, beforeTs, maxPerRequest)

		var resp OHLCVResponse
		_, err := c.requester.Send(&rt.Request{
			URL:    url,
			Method: "GET",
			Headers: map[string]string{
				"Accept": "application/json",
			},
		}, &resp)
		if err != nil {
			return nil, fmt.Errorf("OHLCV request failed: %w", err)
		}

		candles := resp.Data.Attributes.OHLCVList
		if len(candles) == 0 {
			break
		}

		records := c.parseOHLCVList(candles)

		// Filter by start time and collect.
		for _, r := range records {
			if startUnix > 0 && r.OpenTime < startUnix {
				continue
			}
			allRecords = append(allRecords, r)
		}

		if limit > 0 && len(allRecords) >= limit {
			allRecords = allRecords[:limit]
			break
		}

		// The API returns candles sorted descending (newest first).
		// Use the oldest candle's timestamp as the next "before" cursor.
		oldestTs := c.oldestTimestamp(candles)
		if oldestTs <= 0 || oldestTs >= beforeTs {
			break
		}

		// If we got fewer candles than requested, we've reached the beginning.
		if len(candles) < maxPerRequest {
			break
		}

		beforeTs = oldestTs

		// If we've gone past the start time, no need to paginate further.
		if startUnix > 0 && oldestTs <= startUnix {
			break
		}
	}

	return allRecords, nil
}

// parseOHLCVList converts the raw API candle array into OHLCVRecord slice.
// Each candle is [timestamp, open, high, low, close, volume].
func (c *Client) parseOHLCVList(candles [][]interface{}) []trading.OHLCVRecord {
	records := make([]trading.OHLCVRecord, 0, len(candles))

	for _, candle := range candles {
		if len(candle) < 6 {
			continue
		}

		ts, ok := toInt64(candle[0])
		if !ok {
			continue
		}

		records = append(records, trading.OHLCVRecord{
			OpenTime: ts,
			Open:     toStr(candle[1]),
			High:     toStr(candle[2]),
			Low:      toStr(candle[3]),
			Close:    toStr(candle[4]),
			Volume:   toStr(candle[5]),
			IsClosed: true,
		})
	}

	return records
}

// oldestTimestamp returns the smallest timestamp from a raw candle list.
func (c *Client) oldestTimestamp(candles [][]interface{}) int64 {
	var oldest int64
	for _, candle := range candles {
		if len(candle) == 0 {
			continue
		}
		ts, ok := toInt64(candle[0])
		if !ok {
			continue
		}
		if oldest == 0 || ts < oldest {
			oldest = ts
		}
	}
	return oldest
}

// GetTimeframes returns the list of supported timeframes.
func (c *Client) GetTimeframes() []string {
	return []string{"1m", "5m", "15m", "1h", "4h", "6h", "12h", "1D"}
}

// ============================================================================
// Helpers
// ============================================================================

// toInt64 converts a JSON number (float64) to int64.
func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		return i, err == nil
	}
	return 0, false
}

// toStr converts a JSON value to a string representation suitable for OHLCVRecord.
func toStr(v interface{}) string {
	switch n := v.(type) {
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64)
	case string:
		return n
	default:
		return fmt.Sprintf("%v", v)
	}
}
