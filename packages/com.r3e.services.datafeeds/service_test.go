package datafeeds

import (
	"context"
	"testing"
)

func TestService_CreateFeed(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_CreateFeedValidation(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_CreateFeedRequiresRegisteredSigners(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_SubmitUpdate(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_SubmitUpdateSignerVerificationAndAggregation(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_SubmitUpdateUnknownAggregationDefaultsToMedian(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_SubmitUpdateMeanAggregation(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_SubmitUpdateHeartbeatDeviation(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_Lifecycle(t *testing.T) {
	svc := New(nil, nil, nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := svc.Ready(context.Background()); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if svc.Ready(context.Background()) == nil {
		t.Fatalf("expected not ready after stop")
	}
}

func TestService_Manifest(t *testing.T) {
	svc := New(nil, nil, nil)
	m := svc.Manifest()
	if m.Name != "datafeeds" {
		t.Fatalf("expected name datafeeds")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "datafeeds" {
		t.Fatalf("expected name datafeeds")
	}
}

func TestService_Domain(t *testing.T) {
	svc := New(nil, nil, nil)
	if svc.Domain() != "datafeeds" {
		t.Fatalf("expected domain datafeeds")
	}
}

func TestService_GetFeed(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_GetFeedOwnership(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_Publish(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_UpdateFeed(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_UpdateFeed_WrongAccount(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_UpdateFeed_NotFound(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_ListFeeds_MissingAccount(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_ListUpdates_MissingFeed(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_LatestUpdate_NotFound(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}
