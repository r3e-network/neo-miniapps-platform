package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// Config controls the indexer polling loop.
type Config struct {
	RPCURL      string
	Network     string
	StartHeight int64
	PollInterval time.Duration
	DSN         string
	BatchSize   int64
}

func main() {
	var cfg Config
	flag.StringVar(&cfg.RPCURL, "rpc", envDefault("NEO_RPC_URL", "http://localhost:10332"), "NEO RPC endpoint")
	flag.StringVar(&cfg.Network, "network", envDefault("NEO_NETWORK", "mainnet"), "network label (mainnet|testnet)")
	flag.Int64Var(&cfg.StartHeight, "start-height", 0, "height to start indexing from (0 means current height)")
	flag.DurationVar(&cfg.PollInterval, "poll", durationEnv("NEO_POLL_INTERVAL", 5*time.Second), "poll interval for new blocks")
	flag.StringVar(&cfg.DSN, "dsn", envDefault("NEO_INDEXER_DSN", ""), "Postgres DSN for persisting chain data (optional)")
	flag.Int64Var(&cfg.BatchSize, "batch", 50, "max blocks to process per tick")
	flag.Parse()

	log.Printf("neo-indexer (network=%s rpc=%s start=%d poll=%s batch=%d)", cfg.Network, cfg.RPCURL, cfg.StartHeight, cfg.PollInterval, cfg.BatchSize)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newRPCClient(cfg.RPCURL)

	var db *sql.DB
	if cfg.DSN != "" {
		var err error
		db, err = sql.Open("postgres", cfg.DSN)
		if err != nil {
			log.Fatalf("open db: %v", err)
		}
		if err := db.PingContext(ctx); err != nil {
			log.Fatalf("ping db: %v", err)
		}
		if err := ensureSchema(ctx, db); err != nil {
			log.Fatalf("ensure schema: %v", err)
		}
		defer db.Close()
	}

	height := cfg.StartHeight
	if height == 0 {
		chainHeight, err := client.getBlockCount(ctx)
		if err != nil {
			log.Fatalf("fetch chain height: %v", err)
		}
		height = chainHeight - 1
	}

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			chainHeight, err := client.getBlockCount(ctx)
			if err != nil {
			log.Printf("neo-indexer: failed to fetch height: %v", err)
			continue
		}
			maxHeight := chainHeight
			if cfg.BatchSize > 0 && (height+cfg.BatchSize) < chainHeight {
				maxHeight = height + cfg.BatchSize
			}
			for height < maxHeight {
				hash, err := client.getBlockHash(ctx, height)
				if err != nil {
					log.Printf("get block hash height=%d: %v", height, err)
					break
				}
				header, err := client.getBlock(ctx, hash)
				if err != nil {
					log.Printf("get block %s: %v", hash, err)
					break
				}
				log.Printf("indexed height=%d hash=%s txs=%d", height, hash, len(header.Tx))
				if db != nil {
					if err := upsertBlock(ctx, db, height, hash); err != nil {
						log.Printf("persist block height=%d: %v", height, err)
						break
					}
					if err := upsertTransactions(ctx, db, client, height, header.Tx); err != nil {
						log.Printf("persist txs height=%d: %v", height, err)
						break
					}
				}
				height++
			}
		case <-ctx.Done():
			return
		}
	}
}

func envDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func durationEnv(key string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

type rpcClient struct {
	url string
}

func newRPCClient(url string) *rpcClient {
	return &rpcClient{url: strings.TrimSpace(url)}
}

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type blockHeader struct {
	Hash   string      `json:"hash"`
	Index  int64       `json:"index"`
	Size   int64       `json:"size"`
	Time   int64       `json:"time"`
	Prev   string      `json:"previousblockhash"`
	Next   string      `json:"nextblockhash"`
	Tx     []txVerbose `json:"tx"`
}

type txVerbose struct {
	Hash   string `json:"hash"`
	Size   int    `json:"size"`
	Type   string `json:"type"`
	Sender string `json:"sender"`
	NetFee string `json:"netfee"`
	SysFee string `json:"sysfee"`
}

type appLog struct {
	Executions []struct {
		Notifications []struct {
			Contract string      `json:"contract"`
			Event    string      `json:"eventname"`
			State    interface{} `json:"state"`
		} `json:"notifications"`
	} `json:"executions"`
}

func (c *rpcClient) do(ctx context.Context, method string, params []interface{}, out interface{}) error {
	body, _ := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method, Params: params, ID: 1})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if out != nil && rpcResp.Result != nil {
		return json.Unmarshal(rpcResp.Result, out)
	}
	return nil
}

func (c *rpcClient) getBlockCount(ctx context.Context) (int64, error) {
	var count int64
	if err := c.do(ctx, "getblockcount", nil, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func (c *rpcClient) getBlockHash(ctx context.Context, height int64) (string, error) {
	var hash string
	if err := c.do(ctx, "getblockhash", []interface{}{height}, &hash); err != nil {
		return "", err
	}
	return hash, nil
}

func (c *rpcClient) getBlock(ctx context.Context, hash string) (blockHeader, error) {
	var header blockHeader
	if err := c.do(ctx, "getblock", []interface{}{hash, 1}, &header); err != nil {
		return blockHeader{}, err
	}
	return header, nil
}

func (c *rpcClient) getApplicationLog(ctx context.Context, hash string) (appLog, error) {
	var logResp appLog
	if err := c.do(ctx, "getapplicationlog", []interface{}{hash}, &logResp); err != nil {
		return appLog{}, err
	}
	return logResp, nil
}

func ensureSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS neo_blocks (
			height BIGINT PRIMARY KEY,
			hash TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS neo_transactions (
			hash TEXT PRIMARY KEY,
			height BIGINT NOT NULL REFERENCES neo_blocks(height) ON DELETE CASCADE,
			ordinal INTEGER NOT NULL,
			type TEXT,
			sender TEXT,
			net_fee NUMERIC,
			sys_fee NUMERIC,
			size INTEGER,
			raw JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS neo_notifications (
			id BIGSERIAL PRIMARY KEY,
			tx_hash TEXT NOT NULL REFERENCES neo_transactions(hash) ON DELETE CASCADE,
			contract TEXT,
			event TEXT,
			state JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	return err
}

func upsertBlock(ctx context.Context, db *sql.DB, height int64, hash string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO neo_blocks (height, hash)
		VALUES ($1, $2)
		ON CONFLICT (height) DO UPDATE SET hash = EXCLUDED.hash
	`, height, hash)
	return err
}

func upsertTransactions(ctx context.Context, db *sql.DB, client *rpcClient, height int64, txs []txVerbose) error {
	for idx, tx := range txs {
		netFee, _ := strconv.ParseFloat(tx.NetFee, 64)
		sysFee, _ := strconv.ParseFloat(tx.SysFee, 64)
		if _, err := db.ExecContext(ctx, `
			INSERT INTO neo_transactions (hash, height, ordinal, type, sender, net_fee, sys_fee, size, raw)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (hash) DO UPDATE SET height = EXCLUDED.height, ordinal = EXCLUDED.ordinal
		`, tx.Hash, height, idx, tx.Type, tx.Sender, netFee, sysFee, tx.Size, toJSONB(tx)); err != nil {
			return err
		}
		appLog, err := client.getApplicationLog(ctx, tx.Hash)
		if err != nil {
			log.Printf("warning: application log missing for tx %s: %v", tx.Hash, err)
			continue
		}
		if err := persistNotifications(ctx, db, tx.Hash, appLog); err != nil {
			return err
		}
	}
	return nil
}

func toJSONB(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func persistNotifications(ctx context.Context, db *sql.DB, txHash string, log appLog) error {
	for _, exec := range log.Executions {
		for _, n := range exec.Notifications {
			if _, err := db.ExecContext(ctx, `
				INSERT INTO neo_notifications (tx_hash, contract, event, state)
				VALUES ($1, $2, $3, $4)
			`, txHash, n.Contract, n.Event, toJSONB(n.State)); err != nil {
				return err
			}
		}
	}
	return nil
}
