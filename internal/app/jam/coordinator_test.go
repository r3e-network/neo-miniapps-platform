package jam

import (
	"context"
	"errors"
	"testing"
)

func TestErr_Error(t *testing.T) {
	e := Err("test error")
	if e.Error() != "test error" {
		t.Errorf("Error() = %s, want test error", e.Error())
	}
}

func TestErrInvalidCoordinator(t *testing.T) {
	if ErrInvalidCoordinator.Error() == "" {
		t.Error("ErrInvalidCoordinator should have non-empty message")
	}
}

func TestCoordinator_ProcessNext_NilStore(t *testing.T) {
	c := Coordinator{
		Store:  nil,
		Engine: Engine{},
	}

	ok, err := c.ProcessNext(context.Background())
	if ok {
		t.Error("expected ok=false with nil store")
	}
	if !errors.Is(err, ErrInvalidCoordinator) {
		t.Errorf("expected ErrInvalidCoordinator, got %v", err)
	}
}

func TestCoordinator_ProcessNext_NoPending(t *testing.T) {
	store := NewInMemoryStore()
	c := Coordinator{
		Store:  store,
		Engine: Engine{},
	}

	ok, err := c.ProcessNext(context.Background())
	if ok {
		t.Error("expected ok=false with no pending packages")
	}
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCoordinator_ProcessNext_EngineValidationError(t *testing.T) {
	store := NewInMemoryStore()
	pkg := WorkPackage{
		ID:        "pkg-1",
		ServiceID: "svc-1",
		Items:     []WorkItem{{ID: "item-1", PackageID: "pkg-1", Kind: "test", ParamsHash: "hash"}},
	}
	store.EnqueuePackage(context.Background(), pkg)

	// Engine without required fields should fail validation
	c := Coordinator{
		Store:  store,
		Engine: Engine{}, // empty engine fails validation
	}

	ok, err := c.ProcessNext(context.Background())
	if !ok {
		t.Error("expected ok=true (package was found)")
	}
	if err == nil {
		t.Error("expected validation error from engine")
	}
	// Package should be marked as disputed
	updated, _ := store.GetPackage(context.Background(), "pkg-1")
	if updated.Status != PackageStatusDisputed {
		t.Errorf("expected disputed status, got %s", updated.Status)
	}
}

func TestAccumulatorHash_WithInterface(t *testing.T) {
	store := NewInMemoryStore()
	store.SetAccumulatorHash("sha256")
	result := accumulatorHash(store)
	if result != "sha256" {
		t.Errorf("expected sha256, got %s", result)
	}
}

func TestAccumulatorHash_EmptyReturnsDefault(t *testing.T) {
	store := NewInMemoryStore()
	store.SetAccumulatorHash("")
	result := accumulatorHash(store)
	if result != "blake3-256" {
		t.Errorf("expected blake3-256, got %s", result)
	}
}

func TestAccumulatorHash_NoInterface(t *testing.T) {
	result := accumulatorHash(struct{}{})
	if result != "blake3-256" {
		t.Errorf("expected blake3-256 default, got %s", result)
	}
}

func TestInMemoryStore_EnqueueAndNextPending(t *testing.T) {
	store := NewInMemoryStore()

	pkg := WorkPackage{
		ID:        "pkg-1",
		ServiceID: "svc-1",
		Items:     []WorkItem{{ID: "item-1", PackageID: "pkg-1", Kind: "test", ParamsHash: "hash"}},
	}

	if err := store.EnqueuePackage(context.Background(), pkg); err != nil {
		t.Fatalf("EnqueuePackage failed: %v", err)
	}

	next, found, err := store.NextPending(context.Background())
	if err != nil {
		t.Fatalf("NextPending failed: %v", err)
	}
	if !found {
		t.Error("expected to find pending package")
	}
	if next.ID != pkg.ID {
		t.Errorf("expected package ID %s, got %s", pkg.ID, next.ID)
	}
}

func TestInMemoryStore_UpdatePackageStatus(t *testing.T) {
	store := NewInMemoryStore()
	pkg := WorkPackage{
		ID:        "pkg-1",
		ServiceID: "svc-1",
		Items:     []WorkItem{{ID: "item-1", PackageID: "pkg-1", Kind: "test", ParamsHash: "hash"}},
	}
	store.EnqueuePackage(context.Background(), pkg)

	err := store.UpdatePackageStatus(context.Background(), "pkg-1", PackageStatusApplied)
	if err != nil {
		t.Fatalf("UpdatePackageStatus failed: %v", err)
	}

	updated, _ := store.GetPackage(context.Background(), "pkg-1")
	if updated.Status != PackageStatusApplied {
		t.Errorf("expected applied status, got %s", updated.Status)
	}

	// Non-existent package
	err = store.UpdatePackageStatus(context.Background(), "nonexistent", PackageStatusApplied)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestInMemoryStore_SaveReport(t *testing.T) {
	store := NewInMemoryStore()
	pkg := WorkPackage{
		ID:        "pkg-1",
		ServiceID: "svc-1",
		Items:     []WorkItem{{ID: "item-1", PackageID: "pkg-1", Kind: "test", ParamsHash: "hash"}},
	}
	store.EnqueuePackage(context.Background(), pkg)

	report := WorkReport{
		ID:               "report-1",
		PackageID:        "pkg-1",
		ServiceID:        "svc-1",
		RefineOutputHash: "hash123",
	}
	attns := []Attestation{{ReportID: "report-1", WorkerID: "worker-1"}}

	err := store.SaveReport(context.Background(), report, attns)
	if err != nil {
		t.Fatalf("SaveReport failed: %v", err)
	}

	// Retrieve report
	retrieved, retrievedAttns, err := store.GetReportByPackage(context.Background(), "pkg-1")
	if err != nil {
		t.Fatalf("GetReportByPackage failed: %v", err)
	}
	if retrieved.ID != report.ID {
		t.Errorf("expected report ID %s, got %s", report.ID, retrieved.ID)
	}
	if len(retrievedAttns) != 1 {
		t.Errorf("expected 1 attestation, got %d", len(retrievedAttns))
	}
}

func TestInMemoryStore_ListPackages(t *testing.T) {
	store := NewInMemoryStore()
	for i := 0; i < 5; i++ {
		pkgID := "pkg-" + string(rune('1'+i))
		pkg := WorkPackage{
			ID:        pkgID,
			ServiceID: "svc-1",
			Items:     []WorkItem{{ID: "item-" + string(rune('1'+i)), PackageID: pkgID, Kind: "test", ParamsHash: "hash"}},
		}
		store.EnqueuePackage(context.Background(), pkg)
	}

	// List all
	all, _ := store.ListPackages(context.Background(), PackageFilter{})
	if len(all) != 5 {
		t.Errorf("expected 5 packages, got %d", len(all))
	}

	// List with limit
	limited, _ := store.ListPackages(context.Background(), PackageFilter{Limit: 2})
	if len(limited) != 2 {
		t.Errorf("expected 2 packages, got %d", len(limited))
	}

	// List by service
	byService, _ := store.ListPackages(context.Background(), PackageFilter{ServiceID: "svc-1"})
	if len(byService) != 5 {
		t.Errorf("expected 5 packages for service, got %d", len(byService))
	}
}

func TestInMemoryStore_PendingCount(t *testing.T) {
	store := NewInMemoryStore()

	count, _ := store.PendingCount(context.Background())
	if count != 0 {
		t.Errorf("expected 0 pending, got %d", count)
	}

	pkg := WorkPackage{
		ID:        "pkg-1",
		ServiceID: "svc-1",
		Items:     []WorkItem{{ID: "item-1", PackageID: "pkg-1", Kind: "test", ParamsHash: "hash"}},
	}
	store.EnqueuePackage(context.Background(), pkg)

	count, _ = store.PendingCount(context.Background())
	if count != 1 {
		t.Errorf("expected 1 pending, got %d", count)
	}

	store.UpdatePackageStatus(context.Background(), "pkg-1", PackageStatusApplied)
	count, _ = store.PendingCount(context.Background())
	if count != 0 {
		t.Errorf("expected 0 pending after update, got %d", count)
	}
}

func TestInMemoryStore_Accumulators(t *testing.T) {
	store := NewInMemoryStore()
	store.SetAccumulatorsEnabled(true)

	input := ReceiptInput{
		Hash:      "hash123",
		ServiceID: "svc-1",
		EntryType: ReceiptTypePackage,
		Status:    "applied",
	}

	receipt, err := store.AppendReceipt(context.Background(), input)
	if err != nil {
		t.Fatalf("AppendReceipt failed: %v", err)
	}
	if receipt.Seq != 1 {
		t.Errorf("expected seq 1, got %d", receipt.Seq)
	}

	// Get root
	root, _ := store.AccumulatorRoot(context.Background(), "svc-1")
	if root.Seq != 1 {
		t.Errorf("expected root seq 1, got %d", root.Seq)
	}

	// List roots
	roots, _ := store.AccumulatorRoots(context.Background())
	if len(roots) != 1 {
		t.Errorf("expected 1 root, got %d", len(roots))
	}

	// Get receipt
	retrieved, err := store.Receipt(context.Background(), "hash123")
	if err != nil {
		t.Fatalf("Receipt failed: %v", err)
	}
	if retrieved.Hash != "hash123" {
		t.Errorf("expected hash hash123, got %s", retrieved.Hash)
	}
}

func TestInMemoryStore_AccumulatorsDisabled(t *testing.T) {
	store := NewInMemoryStore()
	// Accumulators disabled by default

	input := ReceiptInput{
		Hash:      "hash123",
		ServiceID: "svc-1",
		EntryType: ReceiptTypePackage,
	}

	receipt, err := store.AppendReceipt(context.Background(), input)
	if err != nil {
		t.Fatalf("AppendReceipt should not error when disabled: %v", err)
	}
	if receipt.Seq != 0 {
		t.Error("receipt should be empty when disabled")
	}

	// Root should be empty
	root, _ := store.AccumulatorRoot(context.Background(), "svc-1")
	if root.Seq != 0 {
		t.Error("root should have seq 0 when disabled")
	}

	// Receipt should not be found
	_, err = store.Receipt(context.Background(), "hash123")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound when disabled, got %v", err)
	}
}

func TestInMemoryStore_ListReceipts(t *testing.T) {
	store := NewInMemoryStore()
	store.SetAccumulatorsEnabled(true)

	// Add multiple receipts
	for i := 0; i < 3; i++ {
		store.AppendReceipt(context.Background(), ReceiptInput{
			Hash:      "hash" + string(rune('1'+i)),
			ServiceID: "svc-1",
			EntryType: ReceiptTypePackage,
		})
	}

	receipts, _ := store.ListReceipts(context.Background(), ReceiptFilter{})
	if len(receipts) != 3 {
		t.Errorf("expected 3 receipts, got %d", len(receipts))
	}

	// With limit
	limited, _ := store.ListReceipts(context.Background(), ReceiptFilter{Limit: 2})
	if len(limited) != 2 {
		t.Errorf("expected 2 receipts, got %d", len(limited))
	}
}

func TestInMemoryStore_HashAlgorithm(t *testing.T) {
	store := NewInMemoryStore()

	// Default
	if store.HashAlgorithm() != "blake3-256" {
		t.Errorf("expected blake3-256 default, got %s", store.HashAlgorithm())
	}

	// Set custom
	store.SetAccumulatorHash("sha256")
	if store.HashAlgorithm() != "sha256" {
		t.Errorf("expected sha256, got %s", store.HashAlgorithm())
	}

	// Empty string doesn't change
	store.SetAccumulatorHash("")
	if store.HashAlgorithm() != "sha256" {
		t.Errorf("expected sha256 unchanged, got %s", store.HashAlgorithm())
	}
}
