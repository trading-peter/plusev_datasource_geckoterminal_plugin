package geckoterminal

// ============================================================================
// GeckoTerminal API v2 response types
// https://apiguide.geckoterminal.com/
// ============================================================================

// PoolData represents a single pool in API responses.
type PoolData struct {
	ID            string            `json:"id"`   // e.g. "solana_ABC123..."
	Type          string            `json:"type"` // "pool"
	Attributes    PoolAttributes    `json:"attributes"`
	Relationships PoolRelationships `json:"relationships"`
}

// PoolAttributes holds the pool's metadata and stats.
type PoolAttributes struct {
	Name               string `json:"name"`    // e.g. "USA / SOL"
	Address            string `json:"address"` // Pool address on-chain
	BaseTokenPriceUSD  string `json:"base_token_price_usd"`
	QuoteTokenPriceUSD string `json:"quote_token_price_usd"`
	ReserveInUSD       string `json:"reserve_in_usd"`  // Total liquidity
	PoolCreatedAt      string `json:"pool_created_at"` // ISO 8601
}

// PoolRelationships links to base and quote token data.
type PoolRelationships struct {
	BaseToken  RelationshipData `json:"base_token"`
	QuoteToken RelationshipData `json:"quote_token"`
	Dex        RelationshipData `json:"dex"`
}

// RelationshipData wraps a relationship reference.
type RelationshipData struct {
	Data RelationshipID `json:"data"`
}

// RelationshipID is a type + id reference.
type RelationshipID struct {
	ID   string `json:"id"`   // e.g. "solana_So11111111111111111111111111111111111111112"
	Type string `json:"type"` // e.g. "token"
}

// IncludedToken is returned in the "included" array when requested.
type IncludedToken struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes TokenAttributes `json:"attributes"`
}

// TokenAttributes holds token metadata.
type TokenAttributes struct {
	Address  string `json:"address"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

// SearchPoolsResponse is the response from the /search/pools endpoint.
type SearchPoolsResponse struct {
	Data     []PoolData      `json:"data"`
	Included []IncludedToken `json:"included"`
}

// SinglePoolResponse is the response for a single pool lookup.
type SinglePoolResponse struct {
	Data     PoolData        `json:"data"`
	Included []IncludedToken `json:"included"`
}

// OHLCVResponse is the top-level response for OHLCV endpoints.
type OHLCVResponse struct {
	Data OHLCVData `json:"data"`
}

// OHLCVData wraps the OHLCV attributes.
type OHLCVData struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes OHLCVAttributes `json:"attributes"`
}

// OHLCVAttributes holds the candle list.
type OHLCVAttributes struct {
	OHLCVList [][]interface{} `json:"ohlcv_list"`
	// Each entry: [timestamp, open, high, low, close, volume]
	// timestamp is unix seconds (float64), prices are float64
}
