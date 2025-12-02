// Package sandbox - Storage isolation layer.
//
// This implements Android-style storage isolation where each service
// has its own private storage namespace that other services cannot access.
package sandbox

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
)

// =============================================================================
// Isolated Storage (Android Internal Storage equivalent)
// =============================================================================

// IsolatedStorage provides namespace-isolated storage for a service.
// Each service can only access its own namespace.
type IsolatedStorage struct {
	mu        sync.RWMutex
	serviceID string
	namespace string // Derived from serviceID
	backend   StorageBackend
	quota     StorageQuota
	auditor   *SecurityAuditor
}

// StorageBackend is the underlying storage implementation.
type StorageBackend interface {
	Get(ctx context.Context, namespace, key string) ([]byte, error)
	Set(ctx context.Context, namespace, key string, value []byte) error
	Delete(ctx context.Context, namespace, key string) error
	List(ctx context.Context, namespace, prefix string) ([]string, error)
	Size(ctx context.Context, namespace string) (int64, error)
}

// NewIsolatedStorage creates isolated storage for a service.
func NewIsolatedStorage(
	serviceID string,
	backend StorageBackend,
	maxBytes int64,
	auditor *SecurityAuditor,
) *IsolatedStorage {
	return &IsolatedStorage{
		serviceID: serviceID,
		namespace: sanitizeNamespace(serviceID),
		backend:   backend,
		quota: StorageQuota{
			MaxBytes: maxBytes,
		},
		auditor: auditor,
	}
}

// Get retrieves a value from the service's isolated storage.
func (s *IsolatedStorage) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Validate key (prevent path traversal)
	if err := validateKey(key); err != nil {
		s.logAccess(ctx, key, "get", false)
		return nil, err
	}

	s.logAccess(ctx, key, "get", true)
	return s.backend.Get(ctx, s.namespace, key)
}

// Set stores a value in the service's isolated storage.
func (s *IsolatedStorage) Set(ctx context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate key
	if err := validateKey(key); err != nil {
		s.logAccess(ctx, key, "set", false)
		return err
	}

	// Check quota
	currentSize, err := s.backend.Size(ctx, s.namespace)
	if err != nil {
		return fmt.Errorf("failed to check storage size: %w", err)
	}

	if s.quota.MaxBytes > 0 && currentSize+int64(len(value)) > s.quota.MaxBytes {
		s.logAccess(ctx, key, "set", false)
		return &StorageQuotaExceededError{
			ServiceID: s.serviceID,
			Used:      currentSize,
			Max:       s.quota.MaxBytes,
		}
	}

	s.logAccess(ctx, key, "set", true)
	return s.backend.Set(ctx, s.namespace, key, value)
}

// Delete removes a value from the service's isolated storage.
func (s *IsolatedStorage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateKey(key); err != nil {
		s.logAccess(ctx, key, "delete", false)
		return err
	}

	s.logAccess(ctx, key, "delete", true)
	return s.backend.Delete(ctx, s.namespace, key)
}

// List lists keys with the given prefix in the service's isolated storage.
func (s *IsolatedStorage) List(ctx context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := validateKey(prefix); err != nil && prefix != "" {
		s.logAccess(ctx, prefix, "list", false)
		return nil, err
	}

	s.logAccess(ctx, prefix, "list", true)
	return s.backend.List(ctx, s.namespace, prefix)
}

// Quota returns the storage quota information.
func (s *IsolatedStorage) Quota() StorageQuota {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Update used bytes
	size, _ := s.backend.Size(context.Background(), s.namespace)
	return StorageQuota{
		MaxBytes:  s.quota.MaxBytes,
		UsedBytes: size,
	}
}

func (s *IsolatedStorage) logAccess(ctx context.Context, key, action string, allowed bool) {
	if s.auditor != nil {
		resource := fmt.Sprintf("storage:%s/%s", s.namespace, key)
		s.auditor.LogResourceAccess(ctx, s.serviceID, resource, action, allowed)
	}
}

// =============================================================================
// Isolated Database (Android SQLite/Room equivalent)
// =============================================================================

// IsolatedDatabase provides namespace-isolated database access.
// Each service can only access tables with its own prefix.
type IsolatedDatabase struct {
	mu          sync.RWMutex
	serviceID   string
	tablePrefix string
	db          *sql.DB
	auditor     *SecurityAuditor

	// Allowed tables (cached)
	allowedTables map[string]bool
}

// NewIsolatedDatabase creates isolated database access for a service.
func NewIsolatedDatabase(
	serviceID string,
	db *sql.DB,
	auditor *SecurityAuditor,
) *IsolatedDatabase {
	return &IsolatedDatabase{
		serviceID:     serviceID,
		tablePrefix:   sanitizeTablePrefix(serviceID),
		db:            db,
		auditor:       auditor,
		allowedTables: make(map[string]bool),
	}
}

// Query executes a read query with table access validation.
func (d *IsolatedDatabase) Query(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Validate query doesn't access unauthorized tables
	if err := d.validateQuery(query); err != nil {
		d.logAccess(ctx, query, "query", false)
		return nil, err
	}

	d.logAccess(ctx, query, "query", true)

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows)
}

// Exec executes a write query with table access validation.
func (d *IsolatedDatabase) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Validate query doesn't access unauthorized tables
	if err := d.validateQuery(query); err != nil {
		d.logAccess(ctx, query, "exec", false)
		return 0, err
	}

	d.logAccess(ctx, query, "exec", true)

	result, err := d.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// AllowedTables returns the list of tables this service can access.
func (d *IsolatedDatabase) AllowedTables() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	tables := make([]string, 0, len(d.allowedTables))
	for table := range d.allowedTables {
		tables = append(tables, table)
	}
	return tables
}

// RegisterTable registers a table as accessible by this service.
func (d *IsolatedDatabase) RegisterTable(tableName string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.allowedTables[tableName] = true
}

// validateQuery checks if the query only accesses allowed tables.
func (d *IsolatedDatabase) validateQuery(query string) error {
	// Extract table names from query (simplified - production would use SQL parser)
	tables := extractTableNames(query)

	for _, table := range tables {
		// Check if table has service's prefix or is explicitly allowed
		if !strings.HasPrefix(table, d.tablePrefix) && !d.allowedTables[table] {
			return &DatabaseAccessDeniedError{
				ServiceID: d.serviceID,
				Table:     table,
				Reason:    "table not in service namespace",
			}
		}
	}

	return nil
}

func (d *IsolatedDatabase) logAccess(ctx context.Context, query, action string, allowed bool) {
	if d.auditor != nil {
		// Truncate query for logging
		logQuery := query
		if len(logQuery) > 100 {
			logQuery = logQuery[:100] + "..."
		}
		resource := fmt.Sprintf("database:%s", logQuery)
		d.auditor.LogResourceAccess(ctx, d.serviceID, resource, action, allowed)
	}
}

// =============================================================================
// In-Memory Storage Backend
// =============================================================================

// MemoryStorageBackend is an in-memory implementation of StorageBackend.
type MemoryStorageBackend struct {
	mu   sync.RWMutex
	data map[string]map[string][]byte // namespace -> key -> value
}

// NewMemoryStorageBackend creates a new in-memory storage backend.
func NewMemoryStorageBackend() *MemoryStorageBackend {
	return &MemoryStorageBackend{
		data: make(map[string]map[string][]byte),
	}
}

func (m *MemoryStorageBackend) Get(ctx context.Context, namespace, key string) ([]byte, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()

	ns, exists := m.data[namespace]
	if !exists {
		return nil, &StorageKeyNotFoundError{Key: key}
	}

	value, exists := ns[key]
	if !exists {
		return nil, &StorageKeyNotFoundError{Key: key}
	}

	// Return a copy
	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

func (m *MemoryStorageBackend) Set(ctx context.Context, namespace, key string, value []byte) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.data[namespace]; !exists {
		m.data[namespace] = make(map[string][]byte)
	}

	// Store a copy
	stored := make([]byte, len(value))
	copy(stored, value)
	m.data[namespace][key] = stored
	return nil
}

func (m *MemoryStorageBackend) Delete(ctx context.Context, namespace, key string) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()

	if ns, exists := m.data[namespace]; exists {
		delete(ns, key)
	}
	return nil
}

func (m *MemoryStorageBackend) List(ctx context.Context, namespace, prefix string) ([]string, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()

	ns, exists := m.data[namespace]
	if !exists {
		return []string{}, nil
	}

	var keys []string
	for key := range ns {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (m *MemoryStorageBackend) Size(ctx context.Context, namespace string) (int64, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()

	ns, exists := m.data[namespace]
	if !exists {
		return 0, nil
	}

	var size int64
	for _, value := range ns {
		size += int64(len(value))
	}
	return size, nil
}

// =============================================================================
// Utility Functions
// =============================================================================

// sanitizeNamespace converts a service ID to a safe namespace.
func sanitizeNamespace(serviceID string) string {
	// Replace unsafe characters
	ns := strings.ReplaceAll(serviceID, ".", "_")
	ns = strings.ReplaceAll(ns, "-", "_")
	ns = strings.ReplaceAll(ns, "/", "_")
	return strings.ToLower(ns)
}

// sanitizeTablePrefix converts a service ID to a safe table prefix.
func sanitizeTablePrefix(serviceID string) string {
	return sanitizeNamespace(serviceID) + "_"
}

// validateKey checks if a storage key is valid.
func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Prevent path traversal
	if strings.Contains(key, "..") {
		return fmt.Errorf("key cannot contain '..'")
	}

	// Prevent absolute paths
	if strings.HasPrefix(key, "/") {
		return fmt.Errorf("key cannot start with '/'")
	}

	// Prevent namespace escape
	if strings.Contains(key, "::") {
		return fmt.Errorf("key cannot contain '::'")
	}

	return nil
}

// extractTableNames extracts table names from a SQL query with improved security.
// Handles common SQL patterns including subqueries, aliases, and schema prefixes.
func extractTableNames(query string) []string {
	var tables []string
	seen := make(map[string]bool)

	// Normalize query: lowercase and collapse whitespace
	query = strings.ToLower(query)
	query = normalizeWhitespace(query)

	// Remove string literals to avoid false positives (e.g., 'from' in strings)
	query = removeStringLiterals(query)

	// Remove comments
	query = removeSQLComments(query)

	words := strings.Fields(query)

	// Keywords that precede table names
	tableKeywords := map[string]bool{
		"from":   true,
		"join":   true,
		"into":   true,
		"update": true,
		"table":  true, // CREATE TABLE, DROP TABLE, ALTER TABLE
	}

	for i, word := range words {
		if tableKeywords[word] {
			if i+1 < len(words) {
				table := extractTableName(words[i+1])
				if table != "" && !seen[table] {
					tables = append(tables, table)
					seen[table] = true
				}
			}
		}
	}

	return tables
}

// normalizeWhitespace collapses multiple whitespace characters into single spaces.
func normalizeWhitespace(s string) string {
	var result strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				result.WriteRune(' ')
				prevSpace = true
			}
		} else {
			result.WriteRune(r)
			prevSpace = false
		}
	}
	return result.String()
}

// removeStringLiterals removes single and double quoted strings from SQL.
func removeStringLiterals(query string) string {
	var result strings.Builder
	inSingle := false
	inDouble := false
	prev := rune(0)

	for _, r := range query {
		if r == '\'' && !inDouble && prev != '\\' {
			inSingle = !inSingle
			continue
		}
		if r == '"' && !inSingle && prev != '\\' {
			inDouble = !inDouble
			continue
		}
		if !inSingle && !inDouble {
			result.WriteRune(r)
		}
		prev = r
	}
	return result.String()
}

// removeSQLComments removes SQL comments (-- and /* */).
func removeSQLComments(query string) string {
	// Remove single-line comments
	lines := strings.Split(query, "\n")
	var cleaned []string
	for _, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		cleaned = append(cleaned, line)
	}
	query = strings.Join(cleaned, " ")

	// Remove multi-line comments
	for {
		start := strings.Index(query, "/*")
		if start < 0 {
			break
		}
		end := strings.Index(query[start:], "*/")
		if end < 0 {
			query = query[:start]
			break
		}
		query = query[:start] + " " + query[start+end+2:]
	}

	return query
}

// extractTableName cleans and validates a potential table name.
func extractTableName(word string) string {
	// Remove common suffixes and prefixes
	word = strings.TrimSuffix(word, ",")
	word = strings.TrimSuffix(word, ";")
	word = strings.TrimSuffix(word, ")")
	word = strings.TrimPrefix(word, "(")

	// Skip SQL keywords and special characters
	if word == "" || word == "(" || word == ")" || word == "select" || word == "as" {
		return ""
	}

	// Handle schema.table format - extract just the table name
	if idx := strings.LastIndex(word, "."); idx >= 0 {
		word = word[idx+1:]
	}

	// Validate: table names should be alphanumeric with underscores
	for _, r := range word {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return ""
		}
	}

	// Reject if it looks like a SQL keyword
	sqlKeywords := map[string]bool{
		"select": true, "where": true, "and": true, "or": true,
		"order": true, "by": true, "group": true, "having": true,
		"limit": true, "offset": true, "set": true, "values": true,
		"inner": true, "outer": true, "left": true, "right": true,
		"cross": true, "natural": true, "on": true, "using": true,
	}
	if sqlKeywords[word] {
		return ""
	}

	return word
}

// scanRows converts sql.Rows to a slice of maps.
func scanRows(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]any)
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

// =============================================================================
// Errors
// =============================================================================

// StorageQuotaExceededError is returned when storage quota is exceeded.
type StorageQuotaExceededError struct {
	ServiceID string
	Used      int64
	Max       int64
}

func (e *StorageQuotaExceededError) Error() string {
	return fmt.Sprintf("storage quota exceeded for %s: %d/%d bytes", e.ServiceID, e.Used, e.Max)
}

// StorageKeyNotFoundError is returned when a key is not found.
type StorageKeyNotFoundError struct {
	Key string
}

func (e *StorageKeyNotFoundError) Error() string {
	return fmt.Sprintf("storage key not found: %s", e.Key)
}

// DatabaseAccessDeniedError is returned when database access is denied.
type DatabaseAccessDeniedError struct {
	ServiceID string
	Table     string
	Reason    string
}

func (e *DatabaseAccessDeniedError) Error() string {
	return fmt.Sprintf("database access denied for %s on table %s: %s", e.ServiceID, e.Table, e.Reason)
}
