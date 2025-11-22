package jam

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"strings"
)

// PGStore implements PackageStore on PostgreSQL tables.
type PGStore struct {
	DB           *sql.DB
	hashAlg      string
	accumEnabled bool
}

// NewPGStore constructs a PostgreSQL-backed package store.
func NewPGStore(db *sql.DB) *PGStore {
	return &PGStore{DB: db, hashAlg: "blake3-256"}
}

// EnqueuePackage inserts a work package and its items atomically.
func (s *PGStore) EnqueuePackage(ctx context.Context, pkg WorkPackage) error {
	if err := pkg.ValidateBasic(); err != nil {
		return err
	}
	if pkg.CreatedAt.IsZero() {
		pkg.CreatedAt = time.Now().UTC()
	}
	if pkg.Status == "" {
		pkg.Status = PackageStatusPending
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO jam_work_packages
			(id, service_id, created_by, nonce, expiry, signature, preimage_hashes, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, pkg.ID, pkg.ServiceID, pkg.CreatedBy, pkg.Nonce, pkg.Expiry, pkg.Signature, pq.Array(pkg.PreimageHashes), pkg.Status, pkg.CreatedAt)
	if err != nil {
		return err
	}

	for _, item := range pkg.Items {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO jam_work_items
				(id, package_id, kind, params_hash, preimage_hashes, max_fee, memo)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, item.ID, item.PackageID, item.Kind, item.ParamsHash, pq.Array(item.PreimageHashes), item.MaxFee, item.Memo)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// NextPending returns the next pending package and locks it using SKIP LOCKED.
func (s *PGStore) NextPending(ctx context.Context) (WorkPackage, bool, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return WorkPackage{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var pkg WorkPackage
	var preimageHashes []string
	row := tx.QueryRowContext(ctx, `
		SELECT id, service_id, created_by, nonce, expiry, signature, preimage_hashes, status, created_at
		FROM jam_work_packages
		WHERE status = $1
		ORDER BY created_at
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`, PackageStatusPending)
	if err := row.Scan(&pkg.ID, &pkg.ServiceID, &pkg.CreatedBy, &pkg.Nonce, &pkg.Expiry, &pkg.Signature, pq.Array(&preimageHashes), &pkg.Status, &pkg.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WorkPackage{}, false, tx.Commit()
		}
		return WorkPackage{}, false, err
	}
	pkg.PreimageHashes = preimageHashes

	itemRows, err := tx.QueryContext(ctx, `
		SELECT id, package_id, kind, params_hash, preimage_hashes, max_fee, memo
		FROM jam_work_items
		WHERE package_id = $1
	`, pkg.ID)
	if err != nil {
		return WorkPackage{}, false, err
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var it WorkItem
		var ph []string
		if err := itemRows.Scan(&it.ID, &it.PackageID, &it.Kind, &it.ParamsHash, pq.Array(&ph), &it.MaxFee, &it.Memo); err != nil {
			return WorkPackage{}, false, err
		}
		it.PreimageHashes = ph
		pkg.Items = append(pkg.Items, it)
	}
	if err := itemRows.Err(); err != nil {
		return WorkPackage{}, false, err
	}

	if err := tx.Commit(); err != nil {
		return WorkPackage{}, false, err
	}
	return pkg, true, nil
}

// SaveReport stores a work report and associated attestations.
func (s *PGStore) SaveReport(ctx context.Context, report WorkReport, attns []Attestation) error {
	if err := report.ValidateBasic(); err != nil {
		return err
	}
	if report.CreatedAt.IsZero() {
		report.CreatedAt = time.Now().UTC()
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO jam_work_reports
			(id, package_id, service_id, refine_output_hash, refine_output_compact, traces, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, report.ID, report.PackageID, report.ServiceID, report.RefineOutputHash, report.RefineOutputCompact, report.Traces, report.CreatedAt)
	if err != nil {
		return err
	}

	for _, attn := range attns {
		if attn.CreatedAt.IsZero() {
			attn.CreatedAt = time.Now().UTC()
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO jam_attestations
				(report_id, worker_id, signature, weight, created_at, engine, engine_version)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (report_id, worker_id) DO UPDATE
			SET signature = EXCLUDED.signature,
			    weight = EXCLUDED.weight,
			    created_at = EXCLUDED.created_at,
			    engine = EXCLUDED.engine,
			    engine_version = EXCLUDED.engine_version
		`, attn.ReportID, attn.WorkerID, attn.Signature, attn.Weight, attn.CreatedAt, attn.Engine, attn.EngineVersion)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdatePackageStatus updates the status of a package.
func (s *PGStore) UpdatePackageStatus(ctx context.Context, pkgID string, status PackageStatus) error {
	_, err := s.DB.ExecContext(ctx, `
		UPDATE jam_work_packages SET status = $1 WHERE id = $2
	`, status, pkgID)
	return err
}

// PendingCount returns the count of pending packages.
func (s *PGStore) PendingCount(ctx context.Context) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM jam_work_packages WHERE status = $1`, PackageStatusPending).Scan(&count)
	return count, err
}

// GetPackage fetches a package and its items.
func (s *PGStore) GetPackage(ctx context.Context, pkgID string) (WorkPackage, error) {
	var pkg WorkPackage
	var preimageHashes []string
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, service_id, created_by, nonce, expiry, signature, preimage_hashes, status, created_at
		FROM jam_work_packages
		WHERE id = $1
	`, pkgID).Scan(&pkg.ID, &pkg.ServiceID, &pkg.CreatedBy, &pkg.Nonce, &pkg.Expiry, &pkg.Signature, pq.Array(&preimageHashes), &pkg.Status, &pkg.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WorkPackage{}, ErrNotFound
		}
		return WorkPackage{}, err
	}
	pkg.PreimageHashes = preimageHashes

	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, package_id, kind, params_hash, preimage_hashes, max_fee, memo
		FROM jam_work_items
		WHERE package_id = $1
	`, pkg.ID)
	if err != nil {
		return WorkPackage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var it WorkItem
		var ph []string
		if err := rows.Scan(&it.ID, &it.PackageID, &it.Kind, &it.ParamsHash, pq.Array(&ph), &it.MaxFee, &it.Memo); err != nil {
			return WorkPackage{}, err
		}
		it.PreimageHashes = ph
		pkg.Items = append(pkg.Items, it)
	}
	if err := rows.Err(); err != nil {
		return WorkPackage{}, err
	}
	return pkg, nil
}

// GetReportByPackage returns the report and attestations for a package id.
func (s *PGStore) GetReportByPackage(ctx context.Context, pkgID string) (WorkReport, []Attestation, error) {
	var report WorkReport
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, package_id, service_id, refine_output_hash, refine_output_compact, traces, created_at
		FROM jam_work_reports
		WHERE package_id = $1
	`, pkgID).Scan(&report.ID, &report.PackageID, &report.ServiceID, &report.RefineOutputHash, &report.RefineOutputCompact, &report.Traces, &report.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WorkReport{}, nil, ErrNotFound
		}
		return WorkReport{}, nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT report_id, worker_id, signature, weight, created_at, engine, engine_version
		FROM jam_attestations
		WHERE report_id = $1
	`, report.ID)
	if err != nil {
		return WorkReport{}, nil, err
	}
	defer rows.Close()
	var attns []Attestation
	for rows.Next() {
		var a Attestation
		if err := rows.Scan(&a.ReportID, &a.WorkerID, &a.Signature, &a.Weight, &a.CreatedAt, &a.Engine, &a.EngineVersion); err != nil {
			return WorkReport{}, nil, err
		}
		attns = append(attns, a)
	}
	if err := rows.Err(); err != nil {
		return WorkReport{}, nil, err
	}
	return report, attns, nil
}

// ListPackages returns recent packages matching the filter.
func (s *PGStore) ListPackages(ctx context.Context, filter PackageFilter) ([]WorkPackage, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	var args []any
	clauses := []string{"1=1"}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.ServiceID != "" {
		args = append(args, filter.ServiceID)
		clauses = append(clauses, fmt.Sprintf("service_id = $%d", len(args)))
	}
	args = append(args, limit)
	args = append(args, filter.Offset)
	limitIdx := len(args) - 1
	offsetIdx := len(args)
	query := fmt.Sprintf(`
		SELECT id, service_id, created_by, nonce, expiry, signature, preimage_hashes, status, created_at
		FROM jam_work_packages
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(clauses, " AND "), limitIdx, offsetIdx)

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pkgs []WorkPackage
	for rows.Next() {
		var pkg WorkPackage
		var preimages []string
		if err := rows.Scan(&pkg.ID, &pkg.ServiceID, &pkg.CreatedBy, &pkg.Nonce, &pkg.Expiry, &pkg.Signature, pq.Array(&preimages), &pkg.Status, &pkg.CreatedAt); err != nil {
			return nil, err
		}
		pkg.PreimageHashes = preimages
		pkgs = append(pkgs, pkg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Attach items per package.
	if len(pkgs) == 0 {
		return pkgs, nil
	}
	ids := make([]string, len(pkgs))
	for i, pkg := range pkgs {
		ids[i] = pkg.ID
	}
	itemsQuery := `
		SELECT package_id, id, kind, params_hash, preimage_hashes, max_fee, memo
		FROM jam_work_items
		WHERE package_id = ANY($1)
	`
	itemRows, err := s.DB.QueryContext(ctx, itemsQuery, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()
	itemsByPkg := make(map[string][]WorkItem)
	for itemRows.Next() {
		var pkgID string
		var it WorkItem
		var ph []string
		if err := itemRows.Scan(&pkgID, &it.ID, &it.Kind, &it.ParamsHash, pq.Array(&ph), &it.MaxFee, &it.Memo); err != nil {
			return nil, err
		}
		it.PackageID = pkgID
		it.PreimageHashes = ph
		itemsByPkg[pkgID] = append(itemsByPkg[pkgID], it)
	}
	if err := itemRows.Err(); err != nil {
		return nil, err
	}
	for i := range pkgs {
		pkgs[i].Items = itemsByPkg[pkgs[i].ID]
	}
	return pkgs, nil
}

// ListReports returns reports filtered by service with pagination.
func (s *PGStore) ListReports(ctx context.Context, filter ReportFilter) ([]WorkReport, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	var args []any
	clauses := []string{"1=1"}
	if filter.ServiceID != "" {
		args = append(args, filter.ServiceID)
		clauses = append(clauses, fmt.Sprintf("service_id = $%d", len(args)))
	}
	args = append(args, limit)
	args = append(args, filter.Offset)
	limitIdx := len(args) - 1
	offsetIdx := len(args)
	query := fmt.Sprintf(`
		SELECT id, package_id, service_id, refine_output_hash, refine_output_compact, traces, created_at
		FROM jam_work_reports
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(clauses, " AND "), limitIdx, offsetIdx)

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []WorkReport
	for rows.Next() {
		var r WorkReport
		if err := rows.Scan(&r.ID, &r.PackageID, &r.ServiceID, &r.RefineOutputHash, &r.RefineOutputCompact, &r.Traces, &r.CreatedAt); err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return reports, nil
}

// AppendReceipt records a receipt and advances the accumulator root.
func (s *PGStore) AppendReceipt(ctx context.Context, in ReceiptInput) (Receipt, error) {
	if !s.accumEnabled {
		return Receipt{}, nil
	}
	if in.Hash == "" || in.ServiceID == "" || in.EntryType == "" {
		return Receipt{}, Err("missing receipt fields")
	}
	if in.ProcessedAt.IsZero() {
		in.ProcessedAt = time.Now().UTC()
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Receipt{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var prevSeq int64
	var prevRoot string
	row := tx.QueryRowContext(ctx, `
		SELECT seq, root FROM jam_accumulators WHERE service_id = $1 FOR UPDATE
	`, in.ServiceID)
	switch err := row.Scan(&prevSeq, &prevRoot); err {
	case nil:
	case sql.ErrNoRows:
		prevSeq = 0
		prevRoot = ""
	default:
		return Receipt{}, err
	}

	seq := prevSeq + 1
	if in.MetadataHash == "" {
		in.MetadataHash = reportMetadataHash(WorkReport{RefineOutputHash: in.Hash, ServiceID: in.ServiceID}, s.hashAlg)
	}
	newRoot, _ := deriveRoot(prevRoot, in, seq, s.hashAlg)
	rcpt := Receipt{
		Hash:         in.Hash,
		ServiceID:    in.ServiceID,
		EntryType:    in.EntryType,
		Seq:          seq,
		PrevRoot:     prevRoot,
		NewRoot:      newRoot,
		Status:       in.Status,
		ProcessedAt:  in.ProcessedAt,
		MetadataHash: in.MetadataHash,
		Extra:        in.Extra,
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO jam_receipts
			(hash, service_id, entry_type, seq, prev_root, new_root, status, processed_at, metadata_hash, extra)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (hash) DO NOTHING
	`, rcpt.Hash, rcpt.ServiceID, rcpt.EntryType, rcpt.Seq, rcpt.PrevRoot, rcpt.NewRoot, rcpt.Status, rcpt.ProcessedAt, rcpt.MetadataHash, rcpt.Extra); err != nil {
		return Receipt{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO jam_accumulators (service_id, seq, root, updated_at)
		VALUES ($1,$2,$3,NOW())
		ON CONFLICT (service_id) DO UPDATE
		SET seq = EXCLUDED.seq, root = EXCLUDED.root, updated_at = NOW()
	`, rcpt.ServiceID, rcpt.Seq, rcpt.NewRoot); err != nil {
		return Receipt{}, err
	}

	if err := tx.Commit(); err != nil {
		return Receipt{}, err
	}
	return rcpt, nil
}

// AccumulatorRoot returns the latest root for a service.
func (s *PGStore) AccumulatorRoot(ctx context.Context, serviceID string) (AccumulatorRoot, error) {
	if !s.accumEnabled {
		return AccumulatorRoot{ServiceID: serviceID}, nil
	}
	var root AccumulatorRoot
	err := s.DB.QueryRowContext(ctx, `
		SELECT service_id, seq, root, updated_at
		FROM jam_accumulators
		WHERE service_id = $1
	`, serviceID).Scan(&root.ServiceID, &root.Seq, &root.Root, &root.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AccumulatorRoot{ServiceID: serviceID}, nil
		}
		return AccumulatorRoot{}, err
	}
	return root, nil
}

// AccumulatorRoots returns all roots.
func (s *PGStore) AccumulatorRoots(ctx context.Context) ([]AccumulatorRoot, error) {
	if !s.accumEnabled {
		return nil, nil
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT service_id, seq, root, updated_at
		FROM jam_accumulators
		ORDER BY service_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roots []AccumulatorRoot
	for rows.Next() {
		var r AccumulatorRoot
		if err := rows.Scan(&r.ServiceID, &r.Seq, &r.Root, &r.UpdatedAt); err != nil {
			return nil, err
		}
		roots = append(roots, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roots, nil
}

// SetAccumulatorHash sets the hash algorithm used for roots.
func (s *PGStore) SetAccumulatorHash(algo string) {
	algo = strings.TrimSpace(strings.ToLower(algo))
	if algo == "" {
		return
	}
	s.hashAlg = algo
}

// SetAccumulatorsEnabled toggles accumulator persistence.
func (s *PGStore) SetAccumulatorsEnabled(enabled bool) {
	s.accumEnabled = enabled
}

// HashAlgorithm returns the configured accumulator hash algorithm.
func (s *PGStore) HashAlgorithm() string {
	if s.hashAlg == "" {
		return "blake3-256"
	}
	return s.hashAlg
}

// Receipt returns a receipt by hash.
func (s *PGStore) Receipt(ctx context.Context, hash string) (Receipt, error) {
	if !s.accumEnabled {
		return Receipt{}, ErrNotFound
	}
	var rcpt Receipt
	err := s.DB.QueryRowContext(ctx, `
		SELECT hash, service_id, entry_type, seq, prev_root, new_root, status, processed_at, metadata_hash, extra
		FROM jam_receipts
		WHERE hash = $1
	`, hash).Scan(&rcpt.Hash, &rcpt.ServiceID, &rcpt.EntryType, &rcpt.Seq, &rcpt.PrevRoot, &rcpt.NewRoot, &rcpt.Status, &rcpt.ProcessedAt, &rcpt.MetadataHash, &rcpt.Extra)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Receipt{}, ErrNotFound
		}
		return Receipt{}, err
	}
	return rcpt, nil
}

// ListReceipts returns receipts filtered by service with pagination.
func (s *PGStore) ListReceipts(ctx context.Context, filter ReceiptFilter) ([]Receipt, error) {
	if !s.accumEnabled {
		return nil, nil
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	var args []any
	clauses := []string{"1=1"}
	if filter.ServiceID != "" {
		args = append(args, filter.ServiceID)
		clauses = append(clauses, fmt.Sprintf("service_id = $%d", len(args)))
	}
	args = append(args, limit)
	args = append(args, filter.Offset)
	limitIdx := len(args) - 1
	offsetIdx := len(args)
	query := fmt.Sprintf(`
		SELECT hash, service_id, entry_type, seq, prev_root, new_root, status, processed_at, metadata_hash, extra
		FROM jam_receipts
		WHERE %s
		ORDER BY processed_at DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(clauses, " AND "), limitIdx, offsetIdx)

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rcpts []Receipt
	for rows.Next() {
		var rcpt Receipt
		if err := rows.Scan(&rcpt.Hash, &rcpt.ServiceID, &rcpt.EntryType, &rcpt.Seq, &rcpt.PrevRoot, &rcpt.NewRoot, &rcpt.Status, &rcpt.ProcessedAt, &rcpt.MetadataHash, &rcpt.Extra); err != nil {
			return nil, err
		}
		rcpts = append(rcpts, rcpt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rcpts, nil
}
