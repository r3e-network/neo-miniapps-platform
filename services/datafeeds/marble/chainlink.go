// Package datafeeds provides Chainlink price feed integration.
package datafeedsmarble

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// ChainlinkFeedConfig defines a Chainlink price feed configuration.
type ChainlinkFeedConfig struct {
	FeedID   string // e.g., "BTC/USD"
	Address  string // Contract address on Arbitrum
	Decimals int    // Price decimals (usually 8)
}

// ChainlinkClient reads prices from Chainlink price feeds on Arbitrum.
type ChainlinkClient struct {
	rpcURL string
	client *http.Client
	feeds  map[string]*ChainlinkFeedConfig
}

// DefaultArbitrumRPC is the default Arbitrum One RPC endpoint.
const DefaultArbitrumRPC = "https://arb1.arbitrum.io/rpc"

// latestRoundData function selector: keccak256("latestRoundData()")[:4]
const latestRoundDataSelector = "feaf968c"

// ChainlinkFeeds defines the Chainlink price feed addresses on Arbitrum One.
var ChainlinkFeeds = map[string]*ChainlinkFeedConfig{
	"BTC/USD":  {FeedID: "BTC/USD", Address: "0x6ce185860a4963106506C203335A2910413708e9", Decimals: 8},
	"ETH/USD":  {FeedID: "ETH/USD", Address: "0x639Fe6ab55C921f74e7fac1ee960C0B6293ba612", Decimals: 8},
	"LINK/USD": {FeedID: "LINK/USD", Address: "0x86E53CF1B870786351Da77A57575e79CB55812CB", Decimals: 8},
	"SOL/USD":  {FeedID: "SOL/USD", Address: "0x24ceA4b8ce57cdA5058b924B9B9987992450590c", Decimals: 8},
	"BNB/USD":  {FeedID: "BNB/USD", Address: "0x6970460aabF80C5BE983C6b74e5D06dEDCA95D4A", Decimals: 8},
	"DOGE/USD": {FeedID: "DOGE/USD", Address: "0x9A7FB1b3950837a8D9b40517626E11D4127C098C", Decimals: 8},
	"ADA/USD":  {FeedID: "ADA/USD", Address: "0xD9f615A9b820225edbA2d821c4A696a0924051c6", Decimals: 8},
	"AVAX/USD": {FeedID: "AVAX/USD", Address: "0x8bf61728eeDCE2F32c456454d87B5d6eD6150208", Decimals: 8},
	"LTC/USD":  {FeedID: "LTC/USD", Address: "0x5698690a7B7B84F6aa985ef7690A8A7288FBc9c8", Decimals: 8},
	"UNI/USD":  {FeedID: "UNI/USD", Address: "0x9C917083fDb403ab5ADbEC26Ee294f6EcAda2720", Decimals: 8},
	"XRP/USD":  {FeedID: "XRP/USD", Address: "0xB4AD57B52aB9141de9926a3e0C8dc6264c2ef205", Decimals: 8},
}

// NewChainlinkClient creates a new Chainlink client.
func NewChainlinkClient(rpcURL string) (*ChainlinkClient, error) {
	if rpcURL == "" {
		rpcURL = DefaultArbitrumRPC
	}

	return &ChainlinkClient{
		rpcURL: rpcURL,
		client: &http.Client{Timeout: 15 * time.Second},
		feeds:  ChainlinkFeeds,
	}, nil
}

// Close closes the client connection.
func (c *ChainlinkClient) Close() {
	// No-op for HTTP client
}

// HasFeed returns true if Chainlink supports this feed.
func (c *ChainlinkClient) HasFeed(feedID string) bool {
	_, ok := c.feeds[feedID]
	return ok
}

// rpcRequest represents a JSON-RPC request.
type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// rpcResponse represents a JSON-RPC response.
type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      int       `json:"id"`
	Result  string    `json:"result"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// GetPrice fetches the latest price from a Chainlink feed.
func (c *ChainlinkClient) GetPrice(ctx context.Context, feedID string) (float64, int, error) {
	feed, ok := c.feeds[feedID]
	if !ok {
		return 0, 0, fmt.Errorf("chainlink feed not found: %s", feedID)
	}

	// Build eth_call request
	callData := "0x" + latestRoundDataSelector
	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_call",
		Params: []interface{}{
			map[string]string{
				"to":   feed.Address,
				"data": callData,
			},
			"latest",
		},
		ID: 1,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return 0, 0, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, bytes.NewReader(reqBody))
	if err != nil {
		return 0, 0, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return 0, 0, fmt.Errorf("rpc call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("read response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return 0, 0, fmt.Errorf("unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return 0, 0, fmt.Errorf("rpc error: %s", rpcResp.Error.Message)
	}

	// Parse the result - latestRoundData returns (roundId, answer, startedAt, updatedAt, answeredInRound)
	// Each is 32 bytes, answer is at offset 32 (bytes 64-128 in hex, or chars 66-130 with 0x prefix)
	result := strings.TrimPrefix(rpcResp.Result, "0x")
	if len(result) < 128 {
		return 0, 0, fmt.Errorf("invalid response length")
	}

	// answer is the second 32-byte value (position 32-64 bytes = chars 64-128)
	answerHex := result[64:128]
	answerBytes, err := hex.DecodeString(answerHex)
	if err != nil {
		return 0, 0, fmt.Errorf("decode answer: %w", err)
	}

	answer := new(big.Int).SetBytes(answerBytes)

	// Convert to float with decimals
	decimals := feed.Decimals
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
	price := new(big.Float).SetInt(answer)
	price.Quo(price, divisor)

	priceFloat, _ := price.Float64()
	return priceFloat, decimals, nil
}
