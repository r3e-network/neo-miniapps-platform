package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	"github.com/R3E-Network/service_layer/internal/app/domain/automation"
	domainccip "github.com/R3E-Network/service_layer/internal/app/domain/ccip"
	domainconf "github.com/R3E-Network/service_layer/internal/app/domain/confidential"
	domaindf "github.com/R3E-Network/service_layer/internal/app/domain/datafeeds"
	domainlink "github.com/R3E-Network/service_layer/internal/app/domain/datalink"
	domainds "github.com/R3E-Network/service_layer/internal/app/domain/datastreams"
	domaindta "github.com/R3E-Network/service_layer/internal/app/domain/dta"
	"github.com/R3E-Network/service_layer/internal/app/domain/function"
	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	domainvrf "github.com/R3E-Network/service_layer/internal/app/domain/vrf"
)

// Verifies that tenant-aware list queries refuse to return rows whose tenant no longer
// matches the owning account. This is a defense-in-depth guard in addition to HTTP checks.
func TestTenantFiltersAcrossStores(t *testing.T) {
	store, ctx := newTestStore(t)

	acct, err := store.CreateAccount(ctx, account.Account{
		Owner:    "tenant-user",
		Metadata: map[string]string{"tenant": "tenant-a"},
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Functions and automation jobs
	fn, err := store.CreateFunction(ctx, function.Definition{
		AccountID: acct.ID,
		Name:      "fn",
		Source:    "() => 1",
	})
	if err != nil {
		t.Fatalf("create function: %v", err)
	}
	job := automation.Job{AccountID: acct.ID, FunctionID: fn.ID, Name: "job1", Schedule: "@hourly", Enabled: true}
	if _, err := store.CreateAutomationJob(ctx, job); err != nil {
		t.Fatalf("create automation job: %v", err)
	}
	expectListed := func(t *testing.T, msg string, want int, got int) {
		t.Helper()
		if got != want {
			t.Fatalf("%s: expected %d, got %d", msg, want, got)
		}
	}
	if list, _ := store.ListFunctions(ctx, acct.ID); len(list) != 1 {
		t.Fatalf("expected function present")
	}
	if jobs, _ := store.ListAutomationJobs(ctx, acct.ID); len(jobs) != 1 {
		t.Fatalf("expected automation job present")
	}
	// Force tenant mismatch and ensure they disappear.
	if _, err := store.db.ExecContext(ctx, `UPDATE app_functions SET tenant = 'tenant-b' WHERE id = $1`, fn.ID); err != nil {
		t.Fatalf("mismatch function tenant: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE app_automation_jobs SET tenant = 'tenant-b' WHERE account_id = $1`, acct.ID); err != nil {
		t.Fatalf("mismatch automation tenant: %v", err)
	}
	expectListed(t, "functions filtered", 0, len(mustListFunctions(ctx, store, acct.ID)))
	expectListed(t, "automation filtered", 0, len(mustListAutomation(ctx, store, acct.ID)))

	// Data feeds
	feed, err := store.CreateDataFeed(ctx, domaindf.Feed{
		AccountID:    acct.ID,
		Pair:         "eth/usd",
		Decimals:     8,
		Heartbeat:    time.Minute,
		ThresholdPPM: 10,
	})
	if err != nil {
		t.Fatalf("create data feed: %v", err)
	}
	expectListed(t, "datafeeds present", 1, len(mustListDataFeeds(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE chainlink_data_feeds SET tenant = 'tenant-b' WHERE id = $1`, feed.ID); err != nil {
		t.Fatalf("mismatch datafeed tenant: %v", err)
	}
	expectListed(t, "datafeeds filtered", 0, len(mustListDataFeeds(ctx, store, acct.ID)))

	// Price feeds
	pf, err := store.CreatePriceFeed(ctx, pricefeed.Feed{
		AccountID:        acct.ID,
		BaseAsset:        "ETH",
		QuoteAsset:       "USD",
		Pair:             "eth/usd",
		UpdateInterval:   "1m",
		DeviationPercent: 1,
		Heartbeat:        "1m",
		Active:           true,
	})
	if err != nil {
		t.Fatalf("create price feed: %v", err)
	}
	expectListed(t, "pricefeeds present", 1, len(mustListPriceFeeds(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE app_price_feeds SET tenant = 'tenant-b' WHERE id = $1`, pf.ID); err != nil {
		t.Fatalf("mismatch pricefeed tenant: %v", err)
	}
	expectListed(t, "pricefeeds filtered", 0, len(mustListPriceFeeds(ctx, store, acct.ID)))

	// Oracle sources
	src, err := store.CreateDataSource(ctx, oracle.DataSource{
		AccountID: acct.ID,
		Name:      "src",
		URL:       "https://example.com",
		Method:    "GET",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("create oracle source: %v", err)
	}
	expectListed(t, "oracle sources present", 1, len(mustListOracleSources(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE app_oracle_sources SET tenant = 'tenant-b' WHERE id = $1`, src.ID); err != nil {
		t.Fatalf("mismatch oracle source tenant: %v", err)
	}
	expectListed(t, "oracle sources filtered", 0, len(mustListOracleSources(ctx, store, acct.ID)))

	// DataLink channels
	ch, err := store.CreateChannel(ctx, domainlink.Channel{
		AccountID: acct.ID,
		Name:      "ch",
		Endpoint:  "https://endpoint",
		Status:    domainlink.ChannelStatusActive,
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	expectListed(t, "channels present", 1, len(mustListChannels(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE chainlink_datalink_channels SET tenant = 'tenant-b' WHERE id = $1`, ch.ID); err != nil {
		t.Fatalf("mismatch channel tenant: %v", err)
	}
	expectListed(t, "channels filtered", 0, len(mustListChannels(ctx, store, acct.ID)))

	// CCIP lanes
	lane, err := store.CreateLane(ctx, domainccip.Lane{
		AccountID:   acct.ID,
		Name:        "lane",
		SourceChain: "eth",
		DestChain:   "neo",
	})
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}
	expectListed(t, "lanes present", 1, len(mustListLanes(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE app_ccip_lanes SET tenant = 'tenant-b' WHERE id = $1`, lane.ID); err != nil {
		t.Fatalf("mismatch lane tenant: %v", err)
	}
	expectListed(t, "lanes filtered", 0, len(mustListLanes(ctx, store, acct.ID)))

	// VRF keys
	key, err := store.CreateVRFKey(ctx, domainvrf.Key{
		AccountID:     acct.ID,
		PublicKey:     "pk",
		Label:         "k",
		Status:        domainvrf.KeyStatusActive,
		WalletAddress: "0xabc",
	})
	if err != nil {
		t.Fatalf("create vrf key: %v", err)
	}
	expectListed(t, "vrf keys present", 1, len(mustListVRFKeys(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE app_vrf_keys SET tenant = 'tenant-b' WHERE id = $1`, key.ID); err != nil {
		t.Fatalf("mismatch vrf key tenant: %v", err)
	}
	expectListed(t, "vrf keys filtered", 0, len(mustListVRFKeys(ctx, store, acct.ID)))

	// Datastreams
	stream, err := store.CreateStream(ctx, domainds.Stream{
		AccountID:   acct.ID,
		Name:        "ticker",
		Symbol:      "TCKR",
		Description: "demo",
		Frequency:   "1s",
		SLAms:       10,
		Status:      domainds.StreamStatusActive,
	})
	if err != nil {
		t.Fatalf("create datastream: %v", err)
	}
	expectListed(t, "datastream present", 1, len(mustListStreams(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE chainlink_datastreams SET tenant = 'tenant-b' WHERE id = $1`, stream.ID); err != nil {
		t.Fatalf("mismatch stream tenant: %v", err)
	}
	expectListed(t, "datastream filtered", 0, len(mustListStreams(ctx, store, acct.ID)))

	// DTA
	product, err := store.CreateProduct(ctx, domaindta.Product{
		AccountID: acct.ID,
		Name:      "Fund A",
		Symbol:    "FNDA",
		Type:      "open",
		Status:    domaindta.ProductStatusActive,
	})
	if err != nil {
		t.Fatalf("create dta product: %v", err)
	}
	expectListed(t, "dta product present", 1, len(mustListProducts(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE chainlink_dta_products SET tenant = 'tenant-b' WHERE id = $1`, product.ID); err != nil {
		t.Fatalf("mismatch dta product tenant: %v", err)
	}
	expectListed(t, "dta product filtered", 0, len(mustListProducts(ctx, store, acct.ID)))

	// Confidential
	enclave, err := store.CreateEnclave(ctx, domainconf.Enclave{
		AccountID: acct.ID,
		Name:      "enc",
		Endpoint:  "https://enc",
		Status:    domainconf.EnclaveStatusActive,
	})
	if err != nil {
		t.Fatalf("create enclave: %v", err)
	}
	expectListed(t, "conf enclaves present", 1, len(mustListEnclaves(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE confidential_enclaves SET tenant = 'tenant-b' WHERE id = $1`, enclave.ID); err != nil {
		t.Fatalf("mismatch enclave tenant: %v", err)
	}
	expectListed(t, "conf enclaves filtered", 0, len(mustListEnclaves(ctx, store, acct.ID)))

	// Gas bank
	gasAcct, err := store.CreateGasAccount(ctx, gasbank.Account{
		AccountID:     acct.ID,
		WalletAddress: "0xabc123abc123abc123abc123abc123abc123abcd",
		Metadata:      map[string]string{"note": "gas"},
	})
	if err != nil {
		t.Fatalf("create gas account: %v", err)
	}
	tx := gasbank.Transaction{
		AccountID:     gasAcct.ID,
		UserAccountID: acct.ID,
		Type:          gasbank.TransactionDeposit,
		Amount:        1,
		NetAmount:     1,
		Status:        gasbank.StatusCompleted,
		FromAddress:   "wallet-a",
		ToAddress:     "wallet-b",
		CompletedAt:   time.Now().UTC(),
	}
	if _, err := store.CreateGasTransaction(ctx, tx); err != nil {
		t.Fatalf("create gas tx: %v", err)
	}
	expectListed(t, "gas tx present", 1, len(mustListGasTx(ctx, store, gasAcct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE app_gas_accounts SET tenant = 'tenant-b' WHERE id = $1`, gasAcct.ID); err != nil {
		t.Fatalf("mismatch gas account tenant: %v", err)
	}
	expectListed(t, "gas tx filtered", 0, len(mustListGasTx(ctx, store, gasAcct.ID)))
}

func mustListFunctions(ctx context.Context, store *Store, accountID string) []function.Definition {
	list, err := store.ListFunctions(ctx, accountID)
	if err != nil {
		panic(err)
	}
	return list
}

func mustListAutomation(ctx context.Context, store *Store, accountID string) []automation.Job {
	list, err := store.ListAutomationJobs(ctx, accountID)
	if err != nil {
		panic(err)
	}
	return list
}

func mustListDataFeeds(ctx context.Context, store *Store, accountID string) []domaindf.Feed {
	list, err := store.ListDataFeeds(ctx, accountID)
	if err != nil {
		panic(err)
	}
	return list
}

func mustListPriceFeeds(ctx context.Context, store *Store, accountID string) []pricefeed.Feed {
	list, err := store.ListPriceFeeds(ctx, accountID)
	if err != nil {
		panic(err)
	}
	return list
}

func mustListOracleSources(ctx context.Context, store *Store, accountID string) []oracle.DataSource {
	list, err := store.ListDataSources(ctx, accountID)
	if err != nil {
		panic(err)
	}
	return list
}

func mustListChannels(ctx context.Context, store *Store, accountID string) []domainlink.Channel {
	list, err := store.ListChannels(ctx, accountID)
	if err != nil {
		panic(err)
	}
	return list
}

func mustListLanes(ctx context.Context, store *Store, accountID string) []domainccip.Lane {
	list, err := store.ListLanes(ctx, accountID)
	if err != nil {
		panic(err)
	}
	return list
}

func mustListVRFKeys(ctx context.Context, store *Store, accountID string) []domainvrf.Key {
	list, err := store.ListVRFKeys(ctx, accountID)
	if err != nil {
		panic(err)
	}
	return list
}

func mustListGasTx(ctx context.Context, store *Store, gasAccountID string) []gasbank.Transaction {
	list, err := store.ListGasTransactions(ctx, gasAccountID, 10)
	if err != nil {
		panic(err)
	}
	return list
}

func mustListStreams(ctx context.Context, store *Store, accountID string) []domainds.Stream {
	list, err := store.ListStreams(ctx, accountID)
	if err != nil {
		panic(err)
	}
	return list
}

func mustListProducts(ctx context.Context, store *Store, accountID string) []domaindta.Product {
	list, err := store.ListProducts(ctx, accountID)
	if err != nil {
		panic(err)
	}
	return list
}

func mustListEnclaves(ctx context.Context, store *Store, accountID string) []domainconf.Enclave {
	list, err := store.ListEnclaves(ctx, accountID)
	if err != nil {
		panic(err)
	}
	return list
}
