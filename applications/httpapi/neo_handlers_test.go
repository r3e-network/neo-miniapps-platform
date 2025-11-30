package httpapi

import (
	"context"
	"testing"
)

type stubNeoProvider struct {
	status    neoStatus
	blocks    []neoBlock
	detail    neoBlockDetail
	snapshots []neoSnapshot
	err       error
	storage   []neoStorage
	summary   []neoStorageSummary
}

func (s *stubNeoProvider) Status(ctx context.Context) (neoStatus, error) {
	if s.err != nil {
		return neoStatus{}, s.err
	}
	return s.status, nil
}

func (s *stubNeoProvider) ListBlocks(ctx context.Context, limit, offset int) ([]neoBlock, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.blocks, nil
}

func (s *stubNeoProvider) GetBlock(ctx context.Context, height int64) (neoBlockDetail, error) {
	if s.err != nil {
		return neoBlockDetail{}, s.err
	}
	return s.detail, nil
}

func (s *stubNeoProvider) ListSnapshots(ctx context.Context, limit int) ([]neoSnapshot, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.snapshots, nil
}

func (s *stubNeoProvider) GetSnapshot(ctx context.Context, height int64) (neoSnapshot, error) {
	if s.err != nil {
		return neoSnapshot{}, s.err
	}
	if len(s.snapshots) > 0 {
		return s.snapshots[0], nil
	}
	return neoSnapshot{}, nil
}

func (s *stubNeoProvider) ListStorage(ctx context.Context, height int64) ([]neoStorage, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.storage, nil
}

func (s *stubNeoProvider) ListStorageDiff(ctx context.Context, height int64) ([]neoStorageDiff, error) {
	if s.err != nil {
		return nil, s.err
	}
	return nil, nil
}

func (s *stubNeoProvider) StorageSummary(ctx context.Context, height int64) ([]neoStorageSummary, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.summary, nil
}

func (s *stubNeoProvider) SnapshotBundlePath(ctx context.Context, height int64, diff bool) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return "", nil
}

func TestNeoEndpointsUnavailable(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestNeoEndpointsHappyPath(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

// Compile-time check that stubNeoProvider implements neoProvider.
var _ neoProvider = (*stubNeoProvider)(nil)
