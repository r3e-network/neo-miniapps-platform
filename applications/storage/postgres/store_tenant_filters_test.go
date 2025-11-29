package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/domain/account"
	"github.com/R3E-Network/service_layer/domain/automation"
	domainccip "github.com/R3E-Network/service_layer/domain/ccip"
	domainconf "github.com/R3E-Network/service_layer/domain/confidential"
	domaindf "github.com/R3E-Network/service_layer/domain/datafeeds"
	domainlink "github.com/R3E-Network/service_layer/domain/datalink"
	domainds "github.com/R3E-Network/service_layer/domain/datastreams"
	domaindta "github.com/R3E-Network/service_layer/domain/dta"
	"github.com/R3E-Network/service_layer/domain/function"
	"github.com/R3E-Network/service_layer/domain/gasbank"
	"github.com/R3E-Network/service_layer/domain/oracle"
	"github.com/R3E-Network/service_layer/domain/secret"
	domainvrf "github.com/R3E-Network/service_layer/domain/vrf"
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
	// Data feed updates list filtered by tenant
	if _, err := store.CreateDataFeedUpdate(ctx, domaindf.Update{
		AccountID: acct.ID,
		FeedID:    feed.ID,
		RoundID:   1,
		Price:     "10",
		Signer:    "signer",
		Timestamp: time.Now().UTC(),
		Status:    domaindf.UpdateStatusAccepted,
	}); err != nil {
		t.Fatalf("create datafeed update: %v", err)
	}
	expectListed(t, "datafeed updates present", 1, len(mustListDataFeedUpdates(ctx, store, feed.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE chainlink_data_feed_updates SET tenant = 'tenant-b' WHERE feed_id = $1`, feed.ID); err != nil {
		t.Fatalf("mismatch datafeed update tenant: %v", err)
	}
	expectListed(t, "datafeed updates filtered", 0, len(mustListDataFeedUpdates(ctx, store, feed.ID)))

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
	// Delivery listing should also obey tenant
	if _, err := store.CreateDelivery(ctx, domainlink.Delivery{
		AccountID: acct.ID,
		ChannelID: ch.ID,
		Payload:   map[string]any{"data": true},
		Status:    domainlink.DeliveryStatusPending,
		Attempts:  0,
		Metadata:  map[string]string{"note": "dl"},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	expectListed(t, "deliveries present", 1, len(mustListDeliveries(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE chainlink_datalink_deliveries SET tenant = 'tenant-b' WHERE account_id = $1`, acct.ID); err != nil {
		t.Fatalf("mismatch delivery tenant: %v", err)
	}
	expectListed(t, "deliveries filtered", 0, len(mustListDeliveries(ctx, store, acct.ID)))
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
	msg, err := store.CreateMessage(ctx, domainccip.Message{
		AccountID: acct.ID,
		LaneID:    lane.ID,
		Status:    domainccip.MessageStatusPending,
		Payload:   map[string]any{"foo": "bar"},
	})
	if err != nil {
		t.Fatalf("create ccip message: %v", err)
	}
	expectListed(t, "messages present", 1, len(mustListMessages(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE app_ccip_messages SET tenant = 'tenant-b' WHERE id = $1`, msg.ID); err != nil {
		t.Fatalf("mismatch message tenant: %v", err)
	}
	expectListed(t, "messages filtered", 0, len(mustListMessages(ctx, store, acct.ID)))
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
	req, err := store.CreateVRFRequest(ctx, domainvrf.Request{
		AccountID: acct.ID,
		KeyID:     key.ID,
		Consumer:  "c",
		Seed:      "seed",
		Status:    domainvrf.RequestStatusPending,
	})
	if err != nil {
		t.Fatalf("create vrf request: %v", err)
	}
	expectListed(t, "vrf requests present", 1, len(mustListVRFRequests(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE app_vrf_requests SET tenant = 'tenant-b' WHERE id = $1`, req.ID); err != nil {
		t.Fatalf("mismatch vrf request tenant: %v", err)
	}
	expectListed(t, "vrf requests filtered", 0, len(mustListVRFRequests(ctx, store, acct.ID)))
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
	frame, err := store.CreateFrame(ctx, domainds.Frame{
		AccountID: acct.ID,
		StreamID:  stream.ID,
		Sequence:  1,
		Payload:   map[string]any{"p": 1},
		LatencyMS: 1,
		Status:    domainds.FrameStatusOK,
	})
	if err != nil {
		t.Fatalf("create frame: %v", err)
	}
	expectListed(t, "frames present", 1, len(mustListFrames(ctx, store, stream.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE chainlink_datastream_frames SET tenant = 'tenant-b' WHERE id = $1`, frame.ID); err != nil {
		t.Fatalf("mismatch frame tenant: %v", err)
	}
	expectListed(t, "frames filtered", 0, len(mustListFrames(ctx, store, stream.ID)))
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
	order, err := store.CreateOrder(ctx, domaindta.Order{
		AccountID: acct.ID,
		ProductID: product.ID,
		Type:      domaindta.OrderTypeSubscription,
		Amount:    "10",
		Wallet:    "0xabc",
		Status:    domaindta.OrderStatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create dta order: %v", err)
	}
	expectListed(t, "dta orders present", 1, len(mustListOrders(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE chainlink_dta_orders SET tenant = 'tenant-b' WHERE id = $1`, order.ID); err != nil {
		t.Fatalf("mismatch dta order tenant: %v", err)
	}
	expectListed(t, "dta orders filtered", 0, len(mustListOrders(ctx, store, acct.ID)))
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
	att, err := store.CreateAttestation(ctx, domainconf.Attestation{
		AccountID: acct.ID,
		EnclaveID: enclave.ID,
		Report:    "r",
		Status:    "valid",
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create attestation: %v", err)
	}
	expectListed(t, "conf attestations present", 1, len(mustListAttestations(ctx, store, acct.ID, enclave.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE confidential_attestations SET tenant = 'tenant-b' WHERE id = $1`, att.ID); err != nil {
		t.Fatalf("mismatch attestation tenant: %v", err)
	}
	expectListed(t, "conf attestations filtered", 0, len(mustListAttestations(ctx, store, acct.ID, enclave.ID)))
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

	// Secrets
	if _, err := store.CreateSecret(ctx, secret.Secret{
		AccountID: acct.ID,
		Name:      "api-key",
		Value:     "secret",
		ACL:       secret.ACLNone,
	}); err != nil {
		t.Fatalf("create secret: %v", err)
	}
	expectListed(t, "secrets present", 1, len(mustListSecrets(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE app_secrets SET tenant = 'tenant-b' WHERE account_id = $1`, acct.ID); err != nil {
		t.Fatalf("mismatch secret tenant: %v", err)
	}
	expectListed(t, "secrets filtered", 0, len(mustListSecrets(ctx, store, acct.ID)))

	// Workspace wallets
	walletAddr := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := store.CreateWorkspaceWallet(ctx, account.WorkspaceWallet{
		WorkspaceID:   acct.ID,
		WalletAddress: walletAddr,
		Label:         "signer",
		Status:        "active",
	}); err != nil {
		t.Fatalf("create workspace wallet: %v", err)
	}
	expectListed(t, "workspace wallets present", 1, len(mustListWorkspaceWallets(ctx, store, acct.ID)))
	if _, err := store.db.ExecContext(ctx, `UPDATE workspace_wallets SET tenant = 'tenant-b' WHERE workspace_id = $1`, acct.ID); err != nil {
		t.Fatalf("mismatch workspace wallet tenant: %v", err)
	}
	expectListed(t, "workspace wallets filtered", 0, len(mustListWorkspaceWallets(ctx, store, acct.ID)))
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

func mustListDataFeedUpdates(ctx context.Context, store *Store, feedID string) []domaindf.Update {
	list, err := store.ListDataFeedUpdates(ctx, feedID, 10)
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

func mustListDeliveries(ctx context.Context, store *Store, accountID string) []domainlink.Delivery {
	list, err := store.ListDeliveries(ctx, accountID, 10)
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

func mustListMessages(ctx context.Context, store *Store, accountID string) []domainccip.Message {
	list, err := store.ListMessages(ctx, accountID, 10)
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

func mustListVRFRequests(ctx context.Context, store *Store, accountID string) []domainvrf.Request {
	list, err := store.ListVRFRequests(ctx, accountID, 10)
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

func mustListSecrets(ctx context.Context, store *Store, accountID string) []secret.Secret {
	list, err := store.ListSecrets(ctx, accountID)
	if err != nil {
		panic(err)
	}
	return list
}

func mustListWorkspaceWallets(ctx context.Context, store *Store, accountID string) []account.WorkspaceWallet {
	list, err := store.ListWorkspaceWallets(ctx, accountID)
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

func mustListFrames(ctx context.Context, store *Store, streamID string) []domainds.Frame {
	list, err := store.ListFrames(ctx, streamID, 10)
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

func mustListOrders(ctx context.Context, store *Store, accountID string) []domaindta.Order {
	list, err := store.ListOrders(ctx, accountID, 10)
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

func mustListAttestations(ctx context.Context, store *Store, accountID, enclaveID string) []domainconf.Attestation {
	list, err := store.ListAttestations(ctx, accountID, enclaveID, 10)
	if err != nil {
		panic(err)
	}
	return list
}
