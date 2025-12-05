package supabase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// DatabaseClient handles Supabase Database (PostgREST) operations.
type DatabaseClient struct {
	client *Client
}

// From starts a query builder for a table.
func (d *DatabaseClient) From(table string) *QueryBuilder {
	return &QueryBuilder{
		client: d.client,
		table:  table,
		method: "GET",
		columns: "*",
		filters: make([]string, 0),
		headers: make(map[string]string),
	}
}

// RPC calls a Postgres function.
func (d *DatabaseClient) RPC(ctx context.Context, fn string, params interface{}) ([]byte, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	respBody, statusCode, err := d.client.request(ctx, "POST", d.client.restURL+"/rpc/"+fn, body, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	return respBody, nil
}

// RPCWithToken calls a Postgres function with a user token.
func (d *DatabaseClient) RPCWithToken(ctx context.Context, fn string, params interface{}, accessToken string) ([]byte, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	respBody, statusCode, err := d.client.requestWithToken(ctx, "POST", d.client.restURL+"/rpc/"+fn, body, nil, accessToken)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	return respBody, nil
}

// =============================================================================
// Query Builder
// =============================================================================

// QueryBuilder builds and executes database queries.
type QueryBuilder struct {
	client      *Client
	table       string
	method      string
	columns     string
	filters     []string
	orders      []string
	limitVal    *int
	offsetVal   *int
	body        []byte
	headers     map[string]string
	single      bool
	count       string // "", "exact", "planned", "estimated"
	accessToken string
}

// Select specifies columns to select.
func (q *QueryBuilder) Select(columns string) *QueryBuilder {
	q.method = "GET"
	q.columns = columns
	return q
}

// Insert inserts records.
func (q *QueryBuilder) Insert(data interface{}) *QueryBuilder {
	q.method = "POST"
	body, _ := json.Marshal(data)
	q.body = body
	q.headers["Prefer"] = "return=representation"
	return q
}

// Upsert upserts records.
func (q *QueryBuilder) Upsert(data interface{}, onConflict string) *QueryBuilder {
	q.method = "POST"
	body, _ := json.Marshal(data)
	q.body = body
	q.headers["Prefer"] = "return=representation,resolution=merge-duplicates"
	if onConflict != "" {
		q.headers["on-conflict"] = onConflict
	}
	return q
}

// Update updates records.
func (q *QueryBuilder) Update(data interface{}) *QueryBuilder {
	q.method = "PATCH"
	body, _ := json.Marshal(data)
	q.body = body
	q.headers["Prefer"] = "return=representation"
	return q
}

// Delete deletes records.
func (q *QueryBuilder) Delete() *QueryBuilder {
	q.method = "DELETE"
	q.headers["Prefer"] = "return=representation"
	return q
}

// =============================================================================
// Filters
// =============================================================================

// Eq adds an equality filter.
func (q *QueryBuilder) Eq(column string, value interface{}) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=eq.%v", column, value))
	return q
}

// Neq adds a not-equal filter.
func (q *QueryBuilder) Neq(column string, value interface{}) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=neq.%v", column, value))
	return q
}

// Gt adds a greater-than filter.
func (q *QueryBuilder) Gt(column string, value interface{}) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=gt.%v", column, value))
	return q
}

// Gte adds a greater-than-or-equal filter.
func (q *QueryBuilder) Gte(column string, value interface{}) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=gte.%v", column, value))
	return q
}

// Lt adds a less-than filter.
func (q *QueryBuilder) Lt(column string, value interface{}) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=lt.%v", column, value))
	return q
}

// Lte adds a less-than-or-equal filter.
func (q *QueryBuilder) Lte(column string, value interface{}) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=lte.%v", column, value))
	return q
}

// Like adds a LIKE filter.
func (q *QueryBuilder) Like(column, pattern string) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=like.%s", column, url.QueryEscape(pattern)))
	return q
}

// ILike adds a case-insensitive LIKE filter.
func (q *QueryBuilder) ILike(column, pattern string) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=ilike.%s", column, url.QueryEscape(pattern)))
	return q
}

// Is adds an IS filter (for null, true, false).
func (q *QueryBuilder) Is(column string, value interface{}) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=is.%v", column, value))
	return q
}

// In adds an IN filter.
func (q *QueryBuilder) In(column string, values []interface{}) *QueryBuilder {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%v", v)
	}
	q.filters = append(q.filters, fmt.Sprintf("%s=in.(%s)", column, strings.Join(strValues, ",")))
	return q
}

// Contains adds a contains filter (for arrays/ranges).
func (q *QueryBuilder) Contains(column string, value interface{}) *QueryBuilder {
	jsonVal, _ := json.Marshal(value)
	q.filters = append(q.filters, fmt.Sprintf("%s=cs.%s", column, string(jsonVal)))
	return q
}

// ContainedBy adds a contained-by filter.
func (q *QueryBuilder) ContainedBy(column string, value interface{}) *QueryBuilder {
	jsonVal, _ := json.Marshal(value)
	q.filters = append(q.filters, fmt.Sprintf("%s=cd.%s", column, string(jsonVal)))
	return q
}

// Filter adds a raw filter.
func (q *QueryBuilder) Filter(column string, op FilterOperator, value interface{}) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=%s.%v", column, op, value))
	return q
}

// Or adds an OR filter group.
func (q *QueryBuilder) Or(filters string) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("or=(%s)", filters))
	return q
}

// Not negates the next filter.
func (q *QueryBuilder) Not(column string, op FilterOperator, value interface{}) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=not.%s.%v", column, op, value))
	return q
}

// =============================================================================
// Ordering and Pagination
// =============================================================================

// Order adds an order clause.
func (q *QueryBuilder) Order(column string, opts ...OrderDirection) *QueryBuilder {
	dir := OrderAsc
	if len(opts) > 0 {
		dir = opts[0]
	}
	q.orders = append(q.orders, fmt.Sprintf("%s.%s", column, dir))
	return q
}

// Limit sets the maximum number of rows.
func (q *QueryBuilder) Limit(n int) *QueryBuilder {
	q.limitVal = &n
	return q
}

// Offset sets the number of rows to skip.
func (q *QueryBuilder) Offset(n int) *QueryBuilder {
	q.offsetVal = &n
	return q
}

// Range sets both offset and limit.
func (q *QueryBuilder) Range(from, to int) *QueryBuilder {
	q.headers["Range"] = fmt.Sprintf("%d-%d", from, to)
	q.headers["Range-Unit"] = "items"
	return q
}

// Single expects a single row result.
func (q *QueryBuilder) Single() *QueryBuilder {
	q.single = true
	q.headers["Accept"] = "application/vnd.pgrst.object+json"
	return q
}

// Count includes count in response.
func (q *QueryBuilder) Count(countType string) *QueryBuilder {
	q.count = countType
	return q
}

// WithToken sets the access token for RLS.
func (q *QueryBuilder) WithToken(token string) *QueryBuilder {
	q.accessToken = token
	return q
}

// =============================================================================
// Execution
// =============================================================================

// Execute executes the query and returns raw bytes.
func (q *QueryBuilder) Execute(ctx context.Context) ([]byte, error) {
	urlStr := q.buildURL()

	// Add count header if requested
	if q.count != "" {
		q.headers["Prefer"] = appendPrefer(q.headers["Prefer"], "count="+q.count)
	}

	var respBody []byte
	var statusCode int
	var err error

	if q.accessToken != "" {
		respBody, statusCode, err = q.client.requestWithToken(ctx, q.method, urlStr, q.body, q.headers, q.accessToken)
	} else {
		respBody, statusCode, err = q.client.request(ctx, q.method, urlStr, q.body, q.headers)
	}

	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	return respBody, nil
}

// ExecuteInto executes the query and unmarshals into dest.
func (q *QueryBuilder) ExecuteInto(ctx context.Context, dest interface{}) error {
	data, err := q.Execute(ctx)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	return nil
}

// buildURL builds the request URL.
func (q *QueryBuilder) buildURL() string {
	urlStr := q.client.restURL + "/" + url.PathEscape(q.table)

	params := make([]string, 0)

	// Add select columns
	if q.method == "GET" && q.columns != "" {
		params = append(params, "select="+url.QueryEscape(q.columns))
	}

	// Add filters
	params = append(params, q.filters...)

	// Add order
	if len(q.orders) > 0 {
		params = append(params, "order="+strings.Join(q.orders, ","))
	}

	// Add limit
	if q.limitVal != nil {
		params = append(params, fmt.Sprintf("limit=%d", *q.limitVal))
	}

	// Add offset
	if q.offsetVal != nil {
		params = append(params, fmt.Sprintf("offset=%d", *q.offsetVal))
	}

	if len(params) > 0 {
		urlStr += "?" + strings.Join(params, "&")
	}

	return urlStr
}

// appendPrefer appends to the Prefer header.
func appendPrefer(existing, addition string) string {
	if existing == "" {
		return addition
	}
	return existing + "," + addition
}
