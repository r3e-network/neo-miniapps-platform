package jam

import (
	"context"
	"errors"
	"sync"
)

// PackageStore defines persistence for work packages and reports.
type PackageStore interface {
	EnqueuePackage(ctx context.Context, pkg WorkPackage) error
	NextPending(ctx context.Context) (WorkPackage, bool, error)
	SaveReport(ctx context.Context, report WorkReport, attns []Attestation) error
	UpdatePackageStatus(ctx context.Context, pkgID string, status PackageStatus) error
}

// InMemoryStore is a simple, non-durable store for tests and prototyping.
type InMemoryStore struct {
	mu       sync.Mutex
	pkgs     map[string]WorkPackage
	reports  map[string]WorkReport
	attnList map[string][]Attestation
}

// NewInMemoryStore constructs an empty store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		pkgs:     make(map[string]WorkPackage),
		reports:  make(map[string]WorkReport),
		attnList: make(map[string][]Attestation),
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
		return errors.New("package not found")
	}
	pkg.Status = status
	s.pkgs[pkgID] = pkg
	return nil
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
