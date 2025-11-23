package postgres

import (
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domainccip "github.com/R3E-Network/service_layer/internal/app/domain/ccip"
	domainconf "github.com/R3E-Network/service_layer/internal/app/domain/confidential"
	domaincre "github.com/R3E-Network/service_layer/internal/app/domain/cre"
	domaindf "github.com/R3E-Network/service_layer/internal/app/domain/datafeeds"
	domainlink "github.com/R3E-Network/service_layer/internal/app/domain/datalink"
	domainds "github.com/R3E-Network/service_layer/internal/app/domain/datastreams"
	domaindta "github.com/R3E-Network/service_layer/internal/app/domain/dta"
	domainvrf "github.com/R3E-Network/service_layer/internal/app/domain/vrf"
)

func TestStoreChainlinkIntegration(t *testing.T) {
	store, ctx := newTestStore(t)

	acct, err := store.CreateAccount(ctx, account.Account{Owner: "chainlink"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	wallet, err := store.CreateWorkspaceWallet(ctx, account.WorkspaceWallet{
		WorkspaceID:   acct.ID,
		WalletAddress: "0xabc123abc123abc123abc123abc123abc123abcd",
		Label:         "primary",
		Status:        "active",
	})
	if err != nil {
		t.Fatalf("create workspace wallet: %v", err)
	}

	feed := domaindf.Feed{
		AccountID:    acct.ID,
		Pair:         "ETH/USD",
		Decimals:     8,
		Heartbeat:    time.Minute,
		ThresholdPPM: 500,
		Aggregation:  "median",
		Metadata:     map[string]string{"env": "test"},
	}
	feed, err = store.CreateDataFeed(ctx, feed)
	if err != nil {
		t.Fatalf("create data feed: %v", err)
	}
	feed.Description = "primary"
	if _, err := store.UpdateDataFeed(ctx, feed); err != nil {
		t.Fatalf("update data feed: %v", err)
	}
	if _, err := store.CreateDataFeedUpdate(ctx, domaindf.Update{
		AccountID: acct.ID,
		FeedID:    feed.ID,
		RoundID:   1,
		Price:     "101.5",
		Timestamp: time.Now().UTC(),
		Status:    domaindf.UpdateStatusAccepted,
		Signature: "sig",
	}); err != nil {
		t.Fatalf("create data feed update: %v", err)
	}
	if _, err := store.GetLatestDataFeedUpdate(ctx, feed.ID); err != nil {
		t.Fatalf("latest data feed update: %v", err)
	}

	channel := domainlink.Channel{
		AccountID: acct.ID,
		Name:      "provider-1",
		Endpoint:  "https://provider.example",
		Status:    domainlink.ChannelStatusActive,
		SignerSet: []string{wallet.WalletAddress},
		Metadata:  map[string]string{"tier": "gold"},
	}
	channel, err = store.CreateChannel(ctx, channel)
	if err != nil {
		t.Fatalf("create datalink channel: %v", err)
	}
	channel.Status = domainlink.ChannelStatusSuspended
	if _, err := store.UpdateChannel(ctx, channel); err != nil {
		t.Fatalf("update datalink channel: %v", err)
	}
	if _, err := store.CreateDelivery(ctx, domainlink.Delivery{
		AccountID: acct.ID,
		ChannelID: channel.ID,
		Payload:   map[string]any{"payload": true},
		Status:    domainlink.DeliveryStatusPending,
	}); err != nil {
		t.Fatalf("create datalink delivery: %v", err)
	}

	stream := domainds.Stream{
		AccountID:   acct.ID,
		Name:        "ticker",
		Symbol:      "TCKR",
		Description: "demo stream",
		Frequency:   "1s",
		SLAms:       50,
		Status:      domainds.StreamStatusActive,
	}
	stream, err = store.CreateStream(ctx, stream)
	if err != nil {
		t.Fatalf("create datastream: %v", err)
	}
	if _, err := store.CreateFrame(ctx, domainds.Frame{
		AccountID: acct.ID,
		StreamID:  stream.ID,
		Sequence:  1,
		Payload:   map[string]any{"price": 123},
		LatencyMS: 10,
		Status:    domainds.FrameStatusOK,
	}); err != nil {
		t.Fatalf("create datastream frame: %v", err)
	}

	product := domaindta.Product{
		AccountID:       acct.ID,
		Name:            "Fund A",
		Symbol:          "FNDA",
		Type:            "open",
		Status:          domaindta.ProductStatusActive,
		SettlementTerms: "T+1",
	}
	product, err = store.CreateProduct(ctx, product)
	if err != nil {
		t.Fatalf("create dta product: %v", err)
	}
	if _, err := store.CreateOrder(ctx, domaindta.Order{
		AccountID: acct.ID,
		ProductID: product.ID,
		Type:      domaindta.OrderTypeSubscription,
		Amount:    "1000",
		Wallet:    wallet.WalletAddress,
		Status:    domaindta.OrderStatusPending,
	}); err != nil {
		t.Fatalf("create dta order: %v", err)
	}

	enclave := domainconf.Enclave{
		AccountID: acct.ID,
		Name:      "enclave-1",
		Endpoint:  "https://enclave",
		Status:    domainconf.EnclaveStatusActive,
	}
	enclave, err = store.CreateEnclave(ctx, enclave)
	if err != nil {
		t.Fatalf("create enclave: %v", err)
	}
	if _, err := store.CreateSealedKey(ctx, domainconf.SealedKey{
		AccountID: acct.ID,
		EnclaveID: enclave.ID,
		Name:      "default",
		Blob:      []byte("blob"),
	}); err != nil {
		t.Fatalf("create sealed key: %v", err)
	}
	if _, err := store.CreateAttestation(ctx, domainconf.Attestation{
		AccountID: acct.ID,
		EnclaveID: enclave.ID,
		Report:    "attestation-report",
		Status:    "valid",
	}); err != nil {
		t.Fatalf("create attestation: %v", err)
	}

	lane := domainccip.Lane{
		AccountID:     acct.ID,
		Name:          "lane-1",
		SourceChain:   "eth",
		DestChain:     "neo",
		SignerSet:     []string{wallet.WalletAddress},
		AllowedTokens: []string{"eth"},
	}
	lane, err = store.CreateLane(ctx, lane)
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}
	msg, err := store.CreateMessage(ctx, domainccip.Message{
		AccountID: acct.ID,
		LaneID:    lane.ID,
		Status:    domainccip.MessageStatusPending,
		Payload:   map[string]any{"foo": "bar"},
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	msg.Status = domainccip.MessageStatusDelivered
	if _, err := store.UpdateMessage(ctx, msg); err != nil {
		t.Fatalf("update message: %v", err)
	}

	playbook := domaincre.Playbook{
		AccountID: acct.ID,
		Name:      "test-playbook",
		Steps:     []domaincre.Step{{Name: "step-1", Type: domaincre.StepTypeFunctionCall}},
	}
	playbook, err = store.CreatePlaybook(ctx, playbook)
	if err != nil {
		t.Fatalf("create playbook: %v", err)
	}
	exec, err := store.CreateExecutor(ctx, domaincre.Executor{
		AccountID: acct.ID,
		Name:      "exec",
		Type:      "http",
		Endpoint:  "https://runner",
	})
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	if _, err := store.CreateRun(ctx, domaincre.Run{
		AccountID:  acct.ID,
		PlaybookID: playbook.ID,
		ExecutorID: exec.ID,
		Status:     domaincre.RunStatusPending,
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}

	key := domainvrf.Key{
		AccountID:     acct.ID,
		PublicKey:     "pk-1",
		WalletAddress: wallet.WalletAddress,
		Status:        domainvrf.KeyStatusActive,
	}
	key, err = store.CreateVRFKey(ctx, key)
	if err != nil {
		t.Fatalf("create vrf key: %v", err)
	}
	if _, err := store.CreateVRFRequest(ctx, domainvrf.Request{
		AccountID: acct.ID,
		KeyID:     key.ID,
		Consumer:  "consumer-1",
		Seed:      "seed",
		Status:    domainvrf.RequestStatusPending,
	}); err != nil {
		t.Fatalf("create vrf request: %v", err)
	}
}
