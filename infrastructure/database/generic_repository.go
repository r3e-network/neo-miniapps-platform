// Package database provides generic repository helpers for CRUD operations.
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// =============================================================================
// Generic Repository Helpers
// =============================================================================

// GenericOps provides common CRUD operations for service-specific repositories.
// This reduces boilerplate by centralizing common patterns like:
// - JSON marshaling/unmarshaling
// - Error wrapping with context
// - Query construction
//
// Usage:
//
//	func (r *Repository) Create(ctx context.Context, model *Model) error {
//	    return GenericCreate(r.base, ctx, "table_name", model, func(rows []Model) {
//	        if len(rows) > 0 { *model = rows[0] }
//	    })
//	}
type GenericOps struct{}

// GenericCreate inserts a new record and optionally updates the model with returned data.
// T is the model type (e.g., Account, Trigger, RequestRecord).
func GenericCreate[T any](base *Repository, ctx context.Context, table string, model *T, onResult func([]T)) error {
	if model == nil {
		return fmt.Errorf("%s: model cannot be nil", table)
	}

	data, err := base.Request(ctx, "POST", table, model, "")
	if err != nil {
		return fmt.Errorf("create %s: %w", table, err)
	}

	if onResult != nil {
		var rows []T
		if err := json.Unmarshal(data, &rows); err == nil {
			onResult(rows)
		}
	}
	return nil
}

// GenericUpdate updates an existing record by a key field.
func GenericUpdate[T any](base *Repository, ctx context.Context, table, keyField, keyValue string, model *T) error {
	if model == nil {
		return fmt.Errorf("%s: model cannot be nil", table)
	}
	if keyValue == "" {
		return fmt.Errorf("%s: %s cannot be empty", table, keyField)
	}

	query := fmt.Sprintf("%s=eq.%s", keyField, url.QueryEscape(keyValue))
	_, err := base.Request(ctx, "PATCH", table, model, query)
	if err != nil {
		return fmt.Errorf("update %s: %w", table, err)
	}
	return nil
}

// GenericGetByField fetches a single record by a field value.
// Returns NotFoundError if no records match.
func GenericGetByField[T any](base *Repository, ctx context.Context, table, field, value string) (*T, error) {
	if value == "" {
		return nil, fmt.Errorf("%s: %s cannot be empty", table, field)
	}

	query := fmt.Sprintf("%s=eq.%s&limit=1", field, url.QueryEscape(value))
	data, err := base.Request(ctx, "GET", table, nil, query)
	if err != nil {
		return nil, fmt.Errorf("get %s by %s: %w", table, field, err)
	}

	var rows []T
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", table, err)
	}
	if len(rows) == 0 {
		return nil, NewNotFoundError(table, value)
	}
	return &rows[0], nil
}

// GenericList fetches all records from a table.
func GenericList[T any](base *Repository, ctx context.Context, table string) ([]T, error) {
	data, err := base.Request(ctx, "GET", table, nil, "")
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", table, err)
	}

	var rows []T
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", table, err)
	}
	return rows, nil
}

// GenericListByField fetches records matching a field value.
func GenericListByField[T any](base *Repository, ctx context.Context, table, field, value string) ([]T, error) {
	if value == "" {
		return nil, fmt.Errorf("%s: %s cannot be empty", table, field)
	}

	query := fmt.Sprintf("%s=eq.%s", field, url.QueryEscape(value))
	data, err := base.Request(ctx, "GET", table, nil, query)
	if err != nil {
		return nil, fmt.Errorf("list %s by %s: %w", table, field, err)
	}

	var rows []T
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", table, err)
	}
	return rows, nil
}

// GenericListWithQuery fetches records with a custom query string.
func GenericListWithQuery[T any](base *Repository, ctx context.Context, table, query string) ([]T, error) {
	data, err := base.Request(ctx, "GET", table, nil, query)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", table, err)
	}

	var rows []T
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", table, err)
	}
	return rows, nil
}

// GenericDelete deletes a record by a key field.
func GenericDelete(base *Repository, ctx context.Context, table, keyField, keyValue string) error {
	if keyValue == "" {
		return fmt.Errorf("%s: %s cannot be empty", table, keyField)
	}

	query := fmt.Sprintf("%s=eq.%s", keyField, url.QueryEscape(keyValue))
	_, err := base.Request(ctx, "DELETE", table, nil, query)
	if err != nil {
		return fmt.Errorf("delete %s: %w", table, err)
	}
	return nil
}

// GenericUpdateWithQuery updates records matching a custom query string.
// Useful for composite keys where multiple fields must match.
func GenericUpdateWithQuery[T any](base *Repository, ctx context.Context, table, query string, model *T) error {
	if model == nil {
		return fmt.Errorf("%s: model cannot be nil", table)
	}
	if query == "" {
		return fmt.Errorf("%s: query cannot be empty", table)
	}

	_, err := base.Request(ctx, "PATCH", table, model, query)
	if err != nil {
		return fmt.Errorf("update %s: %w", table, err)
	}
	return nil
}

// GenericDeleteWithQuery deletes records matching a custom query string.
// Useful for composite keys where multiple fields must match.
func GenericDeleteWithQuery(base *Repository, ctx context.Context, table, query string) error {
	if query == "" {
		return fmt.Errorf("%s: query cannot be empty", table)
	}

	_, err := base.Request(ctx, "DELETE", table, nil, query)
	if err != nil {
		return fmt.Errorf("delete %s: %w", table, err)
	}
	return nil
}

// =============================================================================
// Query Builder Helpers
// =============================================================================

// QueryBuilder helps construct Supabase REST queries.
type QueryBuilder struct {
	filters []string
	order   string
	limit   int
}

// NewQuery creates a new query builder.
func NewQuery() *QueryBuilder {
	return &QueryBuilder{}
}

// Eq adds an equality filter: field=eq.value
func (q *QueryBuilder) Eq(field, value string) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=eq.%s", field, url.QueryEscape(value)))
	return q
}

// IsNull adds a null check: field=is.null
func (q *QueryBuilder) IsNull(field string) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=is.null", field))
	return q
}

// IsFalse adds a boolean false check: field=eq.false
func (q *QueryBuilder) IsFalse(field string) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=eq.false", field))
	return q
}

// IsTrue adds a boolean true check: field=eq.true
func (q *QueryBuilder) IsTrue(field string) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=eq.true", field))
	return q
}

// Lte adds a less than or equal filter: field=lte.value
func (q *QueryBuilder) Lte(field, value string) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=lte.%s", field, url.QueryEscape(value)))
	return q
}

// Gte adds a greater than or equal filter: field=gte.value
func (q *QueryBuilder) Gte(field, value string) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=gte.%s", field, url.QueryEscape(value)))
	return q
}

// In adds an IN filter: field=in.(value1,value2,...)
// This is useful for batch queries to avoid N+1 problems.
func (q *QueryBuilder) In(field string, values []string) *QueryBuilder {
	if len(values) == 0 {
		return q
	}
	escaped := make([]string, len(values))
	for i, v := range values {
		escaped[i] = url.QueryEscape(v)
	}
	q.filters = append(q.filters, fmt.Sprintf("%s=in.(%s)", field, joinStrings(escaped, ",")))
	return q
}

// joinStrings joins strings with a separator (avoiding strings package import).
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// OrderAsc adds ascending order: order=field.asc
func (q *QueryBuilder) OrderAsc(field string) *QueryBuilder {
	q.order = fmt.Sprintf("order=%s.asc", field)
	return q
}

// OrderDesc adds descending order: order=field.desc
func (q *QueryBuilder) OrderDesc(field string) *QueryBuilder {
	q.order = fmt.Sprintf("order=%s.desc", field)
	return q
}

// Limit sets the result limit.
func (q *QueryBuilder) Limit(n int) *QueryBuilder {
	q.limit = n
	return q
}

// Build constructs the final query string.
func (q *QueryBuilder) Build() string {
	result := ""
	for i, f := range q.filters {
		if i > 0 {
			result += "&"
		}
		result += f
	}
	if q.order != "" {
		if result != "" {
			result += "&"
		}
		result += q.order
	}
	if q.limit > 0 {
		if result != "" {
			result += "&"
		}
		result += fmt.Sprintf("limit=%d", q.limit)
	}
	return result
}
