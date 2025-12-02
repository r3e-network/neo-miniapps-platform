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
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// Config controls the indexer polling loop.
type Config struct {
	RPCURL       string
	Network      string
	StartHeight  int64
	PollInterval time.Duration
	DSN          string
	BatchSize    int64
	StableBuffer int64
}

const (
	metaLastHeightKey = "last_processed_height"
	metaLastHashKey   = "last_processed_hash"
	metaStableHeight  = "stable_height"
	metaStableHashKey = "stable_hash"
)

func main() {
	var cfg Config
	flag.StringVar(&cfg.RPCURL, "rpc", core.EnvDefault("NEO_RPC_URL", "http://localhost:10332"), "NEO RPC endpoint")
	flag.StringVar(&cfg.Network, "network", core.EnvDefault("NEO_NETWORK", "mainnet"), "network label (mainnet|testnet)")
	flag.Int64Var(&cfg.StartHeight, "start-height", 0, "height to start indexing from (0 means current height)")
	flag.DurationVar(&cfg.PollInterval, "poll", core.EnvDuration("NEO_POLL_INTERVAL", 5*time.Second), "poll interval for new blocks")
	flag.StringVar(&cfg.DSN, "dsn", core.EnvDefault("NEO_INDEXER_DSN", ""), "Postgres DSN for persisting chain data (optional)")
	flag.Int64Var(&cfg.BatchSize, "batch", 50, "max blocks to process per tick")
	flag.Int64Var(&cfg.StableBuffer, "stable-buffer", int64(core.EnvInt("NEO_STABLE_BUFFER", 12)), "min blocks behind head to consider stable (also used by API)")
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
		if db != nil {
			if h, err := loadLastHeight(ctx, db); err == nil && h > 0 {
				height = h + 1
				log.Printf("resuming from last processed height %d", h)
			}
		}
		if height == 0 {
			chainHeight, err := client.getBlockCount(ctx)
			if err != nil {
				log.Fatalf("fetch chain height: %v", err)
			}
			height = chainHeight - 1
		}
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
				onChainRoot, rootErr := client.getStateRoot(ctx, height)
				if rootErr != nil {
					log.Printf("get stateroot height=%d: %v", height, rootErr)
				} else if strings.TrimSpace(header.StateRoot) != "" && strings.TrimSpace(onChainRoot) != "" && !strings.EqualFold(header.StateRoot, onChainRoot) {
					log.Printf("warning: header state root %s != on-chain %s at height=%d", header.StateRoot, onChainRoot, height)
				} else if header.StateRoot == "" {
					header.StateRoot = onChainRoot
				}
				log.Printf("indexed height=%d hash=%s txs=%d", height, hash, len(header.Tx))
				if db != nil {
					if err := upsertBlock(ctx, db, height, header); err != nil {
						log.Printf("persist block height=%d: %v", height, err)
						break
					}
					touched, err := upsertTransactions(ctx, db, client, height, header.Tx)
					if err != nil {
						log.Printf("persist txs height=%d: %v", height, err)
						break
					}
					if len(touched) > 0 {
						if err := upsertStorage(ctx, db, client, height, touched); err != nil {
							log.Printf("persist storage height=%d: %v", height, err)
						}
					}
					if err := recordProgress(ctx, db, height, header.Hash); err != nil {
						log.Printf("record progress height=%d: %v", height, err)
					}
					if err := recordStable(ctx, db, cfg.StableBuffer, height, header.Hash); err != nil {
						log.Printf("record stable height=%d: %v", height, err)
					}
				}
				height++
			}
		case <-ctx.Done():
			return
		}
	}
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
	Hash      string      `json:"hash"`
	Index     int64       `json:"index"`
	Size      int64       `json:"size"`
	Time      int64       `json:"time"`
	Prev      string      `json:"previousblockhash"`
	Next      string      `json:"nextblockhash"`
	StateRoot string      `json:"stateroot"`
	Tx        []txVerbose `json:"tx"`
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
		VMState       string         `json:"vmstate"`
		Exception     string         `json:"exception"`
		GasConsumed   string         `json:"gasconsumed"`
		Stack         interface{}    `json:"stack"`
		Notifications []notification `json:"notifications"`
	} `json:"executions"`
}

type notification struct {
	Contract string      `json:"contract"`
	Event    string      `json:"eventname"`
	State    interface{} `json:"state"`
}

type stateEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// kvMap is a helper for diffing storage snapshots.
type kvMap map[string]string

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
	if err := c.do(ctx, "getapplicationlog", []interface{}{hash, "verbose"}, &logResp); err != nil {
		return appLog{}, err
	}
	return logResp, nil
}

func (c *rpcClient) getStateRoot(ctx context.Context, height int64) (string, error) {
	var root struct {
		Hash string `json:"hash"`
	}
	if err := c.do(ctx, "getstateroot", []interface{}{height}, &root); err != nil {
		return "", err
	}
	return root.Hash, nil
}

func (c *rpcClient) getContractStorage(ctx context.Context, contract string) ([]stateEntry, error) {
	var entries []stateEntry
	if err := c.do(ctx, "getcontractstorage", []interface{}{contract}, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func ensureSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS neo_blocks (
			height BIGINT PRIMARY KEY,
			hash TEXT NOT NULL,
			state_root TEXT,
			prev_hash TEXT,
			next_hash TEXT,
			size BIGINT,
			block_time TIMESTAMPTZ,
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
			vm_state TEXT,
			exception TEXT,
			gas_consumed NUMERIC,
			stack JSONB,
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
			exec_index INTEGER NOT NULL DEFAULT 0,
			state JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS neo_transaction_executions (
			tx_hash TEXT NOT NULL REFERENCES neo_transactions(hash) ON DELETE CASCADE,
			exec_index INTEGER NOT NULL,
			vm_state TEXT,
			exception TEXT,
			gas_consumed NUMERIC,
			stack JSONB,
			notifications JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (tx_hash, exec_index)
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS neo_storage (
			height BIGINT NOT NULL,
			contract TEXT NOT NULL,
			kv JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (height, contract)
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS neo_storage_diffs (
			height BIGINT NOT NULL,
			contract TEXT NOT NULL,
			kv_diff JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (height, contract)
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS neo_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	return err
}

func upsertBlock(ctx context.Context, db *sql.DB, height int64, header blockHeader) error {
	if err := detectReorg(ctx, db, height, header.Hash); err != nil {
		return err
	}
	var blockTime sql.NullTime
	if header.Time > 0 {
		blockTime = sql.NullTime{Time: time.Unix(header.Time, 0).UTC(), Valid: true}
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO neo_blocks (height, hash, state_root, prev_hash, next_hash, size, block_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (height) DO UPDATE SET hash = EXCLUDED.hash, state_root = EXCLUDED.state_root, prev_hash = EXCLUDED.prev_hash, next_hash = EXCLUDED.next_hash, size = EXCLUDED.size, block_time = EXCLUDED.block_time
	`, height, header.Hash, sql.NullString{String: header.StateRoot, Valid: header.StateRoot != ""}, nullString(header.Prev), nullString(header.Next), sql.NullInt64{Int64: header.Size, Valid: header.Size > 0}, blockTime)
	return err
}

func upsertTransactions(ctx context.Context, db *sql.DB, client *rpcClient, height int64, txs []txVerbose) (map[string]struct{}, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	touched := make(map[string]struct{})
	for idx, tx := range txs {
		netFee, _ := strconv.ParseFloat(tx.NetFee, 64)
		sysFee, _ := strconv.ParseFloat(tx.SysFee, 64)
		if _, err := db.ExecContext(ctx, `
			INSERT INTO neo_transactions (hash, height, ordinal, type, sender, net_fee, sys_fee, size, vm_state, exception, gas_consumed, stack, raw)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (hash) DO UPDATE SET height = EXCLUDED.height, ordinal = EXCLUDED.ordinal, vm_state = EXCLUDED.vm_state, exception = EXCLUDED.exception, gas_consumed = EXCLUDED.gas_consumed, stack = EXCLUDED.stack
		`, tx.Hash, height, idx, tx.Type, tx.Sender, netFee, sysFee, tx.Size, nil, nil, nil, nil, toJSONB(tx)); err != nil {
			return nil, err
		}
		appLog, err := client.getApplicationLog(ctx, tx.Hash)
		if err != nil {
			log.Printf("warning: application log missing for tx %s: %v", tx.Hash, err)
			continue
		}
		var summaryVM, summaryExc, summaryStack interface{}
		var summaryGas float64
		if len(appLog.Executions) > 0 {
			summaryVM = appLog.Executions[0].VMState
			summaryExc = appLog.Executions[0].Exception
			summaryGas, _ = strconv.ParseFloat(appLog.Executions[0].GasConsumed, 64)
			summaryStack = appLog.Executions[0].Stack
			if _, err := db.ExecContext(ctx, `
				UPDATE neo_transactions
				SET vm_state = $2, exception = $3, gas_consumed = $4, stack = $5
				WHERE hash = $1
			`, tx.Hash, summaryVM, summaryExc, summaryGas, toJSONB(summaryStack)); err != nil {
				return nil, err
			}
		}
		if err := persistExecutions(ctx, db, tx.Hash, appLog.Executions); err != nil {
			return nil, err
		}
		for _, exec := range appLog.Executions {
			for _, n := range exec.Notifications {
				contract := strings.TrimSpace(n.Contract)
				if contract != "" {
					touched[contract] = struct{}{}
				}
			}
		}
	}
	return touched, nil
}

func toJSONB(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func persistExecutions(ctx context.Context, db *sql.DB, txHash string, executions []struct {
	VMState       string         `json:"vmstate"`
	Exception     string         `json:"exception"`
	GasConsumed   string         `json:"gasconsumed"`
	Stack         interface{}    `json:"stack"`
	Notifications []notification `json:"notifications"`
}) error {
	for idx, exec := range executions {
		gasConsumed, _ := strconv.ParseFloat(exec.GasConsumed, 64)
		if _, err := db.ExecContext(ctx, `
			INSERT INTO neo_transaction_executions (tx_hash, exec_index, vm_state, exception, gas_consumed, stack, notifications)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (tx_hash, exec_index) DO UPDATE SET vm_state = EXCLUDED.vm_state, exception = EXCLUDED.exception, gas_consumed = EXCLUDED.gas_consumed, stack = EXCLUDED.stack, notifications = EXCLUDED.notifications
		`, txHash, idx, exec.VMState, exec.Exception, gasConsumed, toJSONB(exec.Stack), toJSONB(exec.Notifications)); err != nil {
			return err
		}
		if err := persistNotifications(ctx, db, txHash, idx, exec.Notifications); err != nil {
			return err
		}
	}
	return nil
}

func persistNotifications(ctx context.Context, db *sql.DB, txHash string, execIndex int, notifications []notification) error {
	for _, n := range notifications {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO neo_notifications (tx_hash, contract, event, exec_index, state)
			VALUES ($1, $2, $3, $4, $5)
		`, txHash, n.Contract, n.Event, execIndex, toJSONB(n.State)); err != nil {
			return err
		}
	}
	return nil
}

func detectReorg(ctx context.Context, db *sql.DB, height int64, hash string) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	var existing string
	err := db.QueryRowContext(ctx, `SELECT hash FROM neo_blocks WHERE height = $1`, height).Scan(&existing)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		return nil
	}
	if existing != hash {
		log.Printf("reorg detected at height=%d old=%s new=%s; rewriting row", height, existing, hash)
		if _, err := db.ExecContext(ctx, `DELETE FROM neo_blocks WHERE height = $1`, height); err != nil {
			return err
		}
	}
	return nil
}

func loadLastHeight(ctx context.Context, db *sql.DB) (int64, error) {
	var height int64
	err := db.QueryRowContext(ctx, `SELECT value::BIGINT FROM neo_meta WHERE key = $1`, metaLastHeightKey).Scan(&height)
	return height, err
}

func recordProgress(ctx context.Context, db *sql.DB, height int64, hash string) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO neo_meta (key, value)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, metaLastHeightKey, strconv.FormatInt(height, 10)); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO neo_meta (key, value)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, metaLastHashKey, hash); err != nil {
		return err
	}
	return nil
}

func recordStable(ctx context.Context, db *sql.DB, buffer int64, latestHeight int64, latestHash string) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if buffer < 0 {
		buffer = 0
	}
	stableHeight := latestHeight - buffer
	if stableHeight < 0 {
		stableHeight = 0
	}
	// Read hash from blocks table if present, else fall back to latest hash for metadata.
	stableHash := latestHash
	var storedHash sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT hash FROM neo_blocks WHERE height = $1`, stableHeight).Scan(&storedHash); err == nil && storedHash.Valid {
		stableHash = storedHash.String
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO neo_meta (key, value)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, metaStableHeight, strconv.FormatInt(stableHeight, 10)); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO neo_meta (key, value)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, metaStableHashKey, stableHash); err != nil {
		return err
	}
	return nil
}

func nullString(v string) sql.NullString {
	v = strings.TrimSpace(v)
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func upsertStorage(ctx context.Context, db *sql.DB, client *rpcClient, height int64, contracts map[string]struct{}) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	for contract := range contracts {
		entries, err := client.getContractStorage(ctx, contract)
		if err != nil {
			log.Printf("warning: fetch storage contract=%s height=%d: %v", contract, height, err)
			continue
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO neo_storage (height, contract, kv)
			VALUES ($1, $2, $3)
			ON CONFLICT (height, contract) DO UPDATE SET kv = EXCLUDED.kv, updated_at = now()
		`, height, contract, toJSONB(entries)); err != nil {
			return err
		}
		if diff, err := diffStorage(ctx, db, height, contract, entries); err == nil && diff != nil {
			if _, err := db.ExecContext(ctx, `
				INSERT INTO neo_storage_diffs (height, contract, kv_diff)
				VALUES ($1, $2, $3)
				ON CONFLICT (height, contract) DO UPDATE SET kv_diff = EXCLUDED.kv_diff, updated_at = now()
			`, height, contract, toJSONB(diff)); err != nil {
				return err
			}
		}
	}
	return nil
}

func diffStorage(ctx context.Context, db *sql.DB, height int64, contract string, current []stateEntry) ([]stateEntry, error) {
	prev, err := loadPreviousStorage(ctx, db, height, contract)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if len(prev) == 0 {
		return current, nil
	}
	prevMap := toKVMap(prev)
	curMap := toKVMap(current)
	var diff []stateEntry
	for k, v := range curMap {
		if prevMap[k] != v {
			diff = append(diff, stateEntry{Key: k, Value: v})
		}
	}
	return diff, nil
}

func toKVMap(entries []stateEntry) kvMap {
	out := make(kvMap, len(entries))
	for _, e := range entries {
		out[e.Key] = e.Value
	}
	return out
}

func loadPreviousStorage(ctx context.Context, db *sql.DB, height int64, contract string) ([]stateEntry, error) {
	var raw []byte
	err := db.QueryRowContext(ctx, `
		SELECT kv FROM neo_storage
		WHERE contract = $1 AND height < $2
		ORDER BY height DESC
		LIMIT 1
	`, contract, height).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var entries []stateEntry
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &entries)
	}
	return entries, nil
}
