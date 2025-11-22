package jam

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// PackageStore defines persistence for work packages and reports.
type PackageStore interface {
	EnqueuePackage(ctx context.Context, pkg WorkPackage) error
	PendingCount(ctx context.Context) (int, error)
	ListPackages(ctx context.Context, filter PackageFilter) ([]WorkPackage, error)
	ListReports(ctx context.Context, filter ReportFilter) ([]WorkReport, error)
	ListReceipts(ctx context.Context, filter ReceiptFilter) ([]Receipt, error)
	NextPending(ctx context.Context) (WorkPackage, bool, error)
	SaveReport(ctx context.Context, report WorkReport, attns []Attestation) error
	UpdatePackageStatus(ctx context.Context, pkgID string, status PackageStatus) error
	GetPackage(ctx context.Context, pkgID string) (WorkPackage, error)
	GetReportByPackage(ctx context.Context, pkgID string) (WorkReport, []Attestation, error)
	AppendReceipt(ctx context.Context, in ReceiptInput) (Receipt, error)
	AccumulatorRoot(ctx context.Context, serviceID string) (AccumulatorRoot, error)
	Receipt(ctx context.Context, hash string) (Receipt, error)
}

// PackageFilter controls list queries.
type PackageFilter struct {
	Status    PackageStatus
	ServiceID string
	Limit     int
	Offset    int
}

// ReportFilter controls report queries.
type ReportFilter struct {
	ServiceID string
	Limit     int
	Offset    int
}

// ReceiptFilter controls receipt queries.
type ReceiptFilter struct {
	ServiceID string
	Limit     int
	Offset    int
}

// InMemoryStore is a simple, non-durable store for tests and prototyping.
type InMemoryStore struct {
	mu          sync.Mutex
	pkgs        map[string]WorkPackage
	reports     map[string]WorkReport
	attnList    map[string][]Attestation
	receipts    map[string]Receipt
	roots       map[string]AccumulatorRoot
	hashAlg     string
	accumEnable bool
}

// NewInMemoryStore constructs an empty store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		pkgs:     make(map[string]WorkPackage),
		reports:  make(map[string]WorkReport),
		attnList: make(map[string][]Attestation),
		receipts: make(map[string]Receipt),
		roots:    make(map[string]AccumulatorRoot),
		hashAlg:  "blake3-256",
	}
}

func (s *InMemoryStore) EnqueuePackage(_ context.Context, pkg WorkPackage) error {
	if err := pkg.ValidateBasic(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pkgs[pkg.ID] = pkg
	return nil
}

// NextPending returns the first pending package, if any.
func (s *InMemoryStore) NextPending(_ context.Context) (WorkPackage, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, pkg := range s.pkgs {
		if pkg.Status == "" || pkg.Status == PackageStatusPending {
			pkg.Status = PackageStatusPending
			s.pkgs[id] = pkg
			return pkg, true, nil
		}
	}
	return WorkPackage{}, false, nil
}

func (s *InMemoryStore) SaveReport(_ context.Context, report WorkReport, attns []Attestation) error {
	if err := report.ValidateBasic(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reports[report.ID] = report
	s.attnList[report.ID] = append([]Attestation(nil), attns...)
	return nil
}

func (s *InMemoryStore) UpdatePackageStatus(_ context.Context, pkgID string, status PackageStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pkg, ok := s.pkgs[pkgID]
	if !ok {
		return ErrNotFound
	}
	pkg.Status = status
	s.pkgs[pkgID] = pkg
	return nil
}

// GetPackage fetches a package by id.
func (s *InMemoryStore) GetPackage(_ context.Context, pkgID string) (WorkPackage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pkg, ok := s.pkgs[pkgID]
	if !ok {
		return WorkPackage{}, ErrNotFound
	}
	return pkg, nil
}

// ListPackages returns packages matching the filter (unsorted).
func (s *InMemoryStore) ListPackages(_ context.Context, filter PackageFilter) ([]WorkPackage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit := filter.Limit
	if limit <= 0 || limit > len(s.pkgs) {
		limit = len(s.pkgs)
	}
	out := make([]WorkPackage, 0, limit)
	offset := filter.Offset
	for _, pkg := range s.pkgs {
		if filter.Status != "" && pkg.Status != filter.Status {
			continue
		}
		if filter.ServiceID != "" && pkg.ServiceID != filter.ServiceID {
			continue
		}
		if offset > 0 {
			offset--
			continue
		}
		out = append(out, pkg)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// GetReportByPackage returns a report and attestations for a package id.
func (s *InMemoryStore) GetReportByPackage(_ context.Context, pkgID string) (WorkReport, []Attestation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var report WorkReport
	var found bool
	for _, r := range s.reports {
		if r.PackageID == pkgID {
			report = r
			found = true
			break
		}
	}
	if !found {
		return WorkReport{}, nil, ErrNotFound
	}
	attns := append([]Attestation(nil), s.attnList[report.ID]...)
	return report, attns, nil
}

// ReportFor returns a stored report for convenience in tests.
func (s *InMemoryStore) ReportFor(pkgID string) (WorkReport, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.reports {
		if r.PackageID == pkgID {
			return r, true
		}
	}
	return WorkReport{}, false
}

// AttestationsFor returns attestations for a report id.
func (s *InMemoryStore) AttestationsFor(reportID string) []Attestation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Attestation(nil), s.attnList[reportID]...)
}

// PendingCount returns number of pending packages.
func (s *InMemoryStore) PendingCount(_ context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, pkg := range s.pkgs {
		if pkg.Status == "" || pkg.Status == PackageStatusPending {
			count++
		}
	}
	return count, nil
}

// ListReports returns reports filtered by service if set.
func (s *InMemoryStore) ListReports(_ context.Context, filter ReportFilter) ([]WorkReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit := filter.Limit
	if limit <= 0 || limit > len(s.reports) {
		limit = len(s.reports)
	}
	offset := filter.Offset
	out := make([]WorkReport, 0, limit)
	for _, r := range s.reports {
		if filter.ServiceID != "" && r.ServiceID != filter.ServiceID {
			continue
		}
		if offset > 0 {
			offset--
			continue
		}
		out = append(out, r)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// AppendReceipt records a receipt and updates the accumulator root.
func (s *InMemoryStore) AppendReceipt(_ context.Context, in ReceiptInput) (Receipt, error) {
	if !s.accumEnable {
		return Receipt{}, nil
	}
	if in.Hash == "" || in.ServiceID == "" || in.EntryType == "" {
		return Receipt{}, Err("missing receipt fields")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	root := s.roots[in.ServiceID]
	seq := root.Seq + 1
	if in.ProcessedAt.IsZero() {
		in.ProcessedAt = time.Now().UTC()
	}
	if in.MetadataHash == "" {
		in.MetadataHash = reportMetadataHash(WorkReport{RefineOutputHash: in.Hash, ServiceID: in.ServiceID}, s.hashAlg)
	}
	newRoot, _ := deriveRoot(root.Root, in, seq, s.hashAlg)
	rcpt := Receipt{
		Hash:         in.Hash,
		ServiceID:    in.ServiceID,
		EntryType:    in.EntryType,
		Seq:          seq,
		PrevRoot:     root.Root,
		NewRoot:      newRoot,
		Status:       in.Status,
		ProcessedAt:  in.ProcessedAt,
		MetadataHash: in.MetadataHash,
		Extra:        in.Extra,
	}
	s.roots[in.ServiceID] = AccumulatorRoot{
		ServiceID: in.ServiceID,
		Seq:       seq,
		Root:      newRoot,
		UpdatedAt: in.ProcessedAt,
	}
	s.receipts[in.Hash] = rcpt
	return rcpt, nil
}

// AccumulatorRoot returns the current root for a service.
func (s *InMemoryStore) AccumulatorRoot(_ context.Context, serviceID string) (AccumulatorRoot, error) {
	if !s.accumEnable {
		return AccumulatorRoot{ServiceID: serviceID}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	root, ok := s.roots[serviceID]
	if !ok {
		return AccumulatorRoot{ServiceID: serviceID}, nil
	}
	return root, nil
}

// Receipt returns a stored receipt by hash.
func (s *InMemoryStore) Receipt(_ context.Context, hash string) (Receipt, error) {
	if !s.accumEnable {
		return Receipt{}, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rcpt, ok := s.receipts[hash]
	if !ok {
		return Receipt{}, ErrNotFound
	}
	return rcpt, nil
}

// ListReceipts returns receipts with basic pagination.
func (s *InMemoryStore) ListReceipts(_ context.Context, filter ReceiptFilter) ([]Receipt, error) {
	if !s.accumEnable {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var all []Receipt
	for _, rcpt := range s.receipts {
		if filter.ServiceID != "" && rcpt.ServiceID != filter.ServiceID {
			continue
		}
		all = append(all, rcpt)
	}
	// Sort by seq descending for deterministic output.
	sort.Slice(all, func(i, j int) bool { return all[i].Seq > all[j].Seq })
	limit := filter.Limit
	if limit <= 0 || limit > len(all) {
		limit = len(all)
	}
	offset := filter.Offset
	if offset < 0 || offset > len(all) {
		offset = 0
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return append([]Receipt(nil), all[offset:end]...), nil
}

// SetAccumulatorHash overrides the hash algorithm used for roots/metadata.
func (s *InMemoryStore) SetAccumulatorHash(algo string) {
	algo = strings.TrimSpace(strings.ToLower(algo))
	if algo == "" {
		return
	}
	s.mu.Lock()
	s.hashAlg = algo
	s.mu.Unlock()
}

// SetAccumulatorsEnabled toggles accumulator persistence.
func (s *InMemoryStore) SetAccumulatorsEnabled(enabled bool) {
	s.mu.Lock()
	s.accumEnable = enabled
	s.mu.Unlock()
}

// HashAlgorithm returns the configured accumulator hash algorithm.
func (s *InMemoryStore) HashAlgorithm() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hashAlg == "" {
		return "blake3-256"
	}
	return s.hashAlg
}
