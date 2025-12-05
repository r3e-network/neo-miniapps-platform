// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// databaseAPIImpl implements DatabaseAPI.
// Uses Supabase REST API for database operations.
type databaseAPIImpl struct {
	ctx       *ServiceContext
	serviceID string
}

func newDatabaseAPI(ctx *ServiceContext, serviceID string) *databaseAPIImpl {
	return &databaseAPIImpl{
		ctx:       ctx,
		serviceID: serviceID,
	}
}

func (d *databaseAPIImpl) From(table string) QueryBuilder {
	return &queryBuilderImpl{
		ctx:   d.ctx,
		table: table,
	}
}

func (d *databaseAPIImpl) RPC(ctx context.Context, fn string, params any) ([]byte, error) {
	if err := d.ctx.RequireCapability(CapDatabase); err != nil {
		return nil, err
	}

	// Build RPC request body
	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal RPC params: %w", err)
	}

	// Use network API to call Supabase RPC endpoint
	// The actual URL will be configured via service config
	req := HTTPRequest{
		Method: "POST",
		URL:    fmt.Sprintf("/rest/v1/rpc/%s", url.PathEscape(fn)),
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		Body: body,
	}

	resp, err := d.ctx.Network().Fetch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("RPC call failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("RPC error: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	return resp.Body, nil
}

// queryBuilderImpl implements QueryBuilder with Supabase PostgREST syntax.
type queryBuilderImpl struct {
	ctx     *ServiceContext
	table   string
	columns string
	method  string
	filters []string
	orders  []string
	limit   *int
	offset  *int
	single  bool
	data    any
}

func (q *queryBuilderImpl) Select(columns string) QueryBuilder {
	q.method = "SELECT"
	q.columns = columns
	return q
}

func (q *queryBuilderImpl) Insert(data any) QueryBuilder {
	q.method = "INSERT"
	q.data = data
	return q
}

func (q *queryBuilderImpl) Update(data any) QueryBuilder {
	q.method = "UPDATE"
	q.data = data
	return q
}

func (q *queryBuilderImpl) Delete() QueryBuilder {
	q.method = "DELETE"
	return q
}

func (q *queryBuilderImpl) Eq(column string, value any) QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=eq.%v", column, value))
	return q
}

func (q *queryBuilderImpl) Neq(column string, value any) QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=neq.%v", column, value))
	return q
}

func (q *queryBuilderImpl) Gt(column string, value any) QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=gt.%v", column, value))
	return q
}

func (q *queryBuilderImpl) Gte(column string, value any) QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=gte.%v", column, value))
	return q
}

func (q *queryBuilderImpl) Lt(column string, value any) QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=lt.%v", column, value))
	return q
}

func (q *queryBuilderImpl) Lte(column string, value any) QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=lte.%v", column, value))
	return q
}

func (q *queryBuilderImpl) Like(column, pattern string) QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=like.%s", column, url.QueryEscape(pattern)))
	return q
}

func (q *queryBuilderImpl) ILike(column, pattern string) QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=ilike.%s", column, url.QueryEscape(pattern)))
	return q
}

func (q *queryBuilderImpl) Is(column string, value any) QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=is.%v", column, value))
	return q
}

func (q *queryBuilderImpl) In(column string, values []any) QueryBuilder {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%v", v)
	}
	q.filters = append(q.filters, fmt.Sprintf("%s=in.(%s)", column, strings.Join(strValues, ",")))
	return q
}

func (q *queryBuilderImpl) Order(column string, ascending bool) QueryBuilder {
	dir := "desc"
	if ascending {
		dir = "asc"
	}
	q.orders = append(q.orders, fmt.Sprintf("%s.%s", column, dir))
	return q
}

func (q *queryBuilderImpl) Limit(n int) QueryBuilder {
	q.limit = &n
	return q
}

func (q *queryBuilderImpl) Offset(n int) QueryBuilder {
	q.offset = &n
	return q
}

func (q *queryBuilderImpl) Single() QueryBuilder {
	q.single = true
	return q
}

func (q *queryBuilderImpl) Execute(ctx context.Context) ([]byte, error) {
	if err := q.ctx.RequireCapability(CapDatabase); err != nil {
		return nil, err
	}

	// Build the request based on method
	var req HTTPRequest
	var err error

	switch q.method {
	case "SELECT", "":
		req, err = q.buildSelectRequest()
	case "INSERT":
		req, err = q.buildInsertRequest()
	case "UPDATE":
		req, err = q.buildUpdateRequest()
	case "DELETE":
		req, err = q.buildDeleteRequest()
	default:
		return nil, fmt.Errorf("unknown method: %s", q.method)
	}

	if err != nil {
		return nil, err
	}

	// Execute the request
	resp, err := q.ctx.Network().Fetch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("database request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("database error: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	return resp.Body, nil
}

func (q *queryBuilderImpl) ExecuteInto(ctx context.Context, dest any) error {
	data, err := q.Execute(ctx)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	return json.Unmarshal(data, dest)
}

// buildSelectRequest builds a GET request for SELECT operations.
func (q *queryBuilderImpl) buildSelectRequest() (HTTPRequest, error) {
	path := fmt.Sprintf("/rest/v1/%s", url.PathEscape(q.table))

	// Build query string
	params := url.Values{}

	if q.columns != "" {
		params.Set("select", q.columns)
	}

	for _, filter := range q.filters {
		parts := strings.SplitN(filter, "=", 2)
		if len(parts) == 2 {
			params.Add(parts[0], parts[1])
		}
	}

	if len(q.orders) > 0 {
		params.Set("order", strings.Join(q.orders, ","))
	}

	if q.limit != nil {
		params.Set("limit", fmt.Sprintf("%d", *q.limit))
	}

	if q.offset != nil {
		params.Set("offset", fmt.Sprintf("%d", *q.offset))
	}

	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}

	headers := map[string]string{
		"Accept": "application/json",
	}

	if q.single {
		headers["Accept"] = "application/vnd.pgrst.object+json"
	}

	return HTTPRequest{
		Method:  "GET",
		URL:     path,
		Headers: headers,
	}, nil
}

// buildInsertRequest builds a POST request for INSERT operations.
func (q *queryBuilderImpl) buildInsertRequest() (HTTPRequest, error) {
	path := fmt.Sprintf("/rest/v1/%s", url.PathEscape(q.table))

	body, err := json.Marshal(q.data)
	if err != nil {
		return HTTPRequest{}, fmt.Errorf("failed to marshal insert data: %w", err)
	}

	return HTTPRequest{
		Method: "POST",
		URL:    path,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
			"Prefer":       "return=representation",
		},
		Body: body,
	}, nil
}

// buildUpdateRequest builds a PATCH request for UPDATE operations.
func (q *queryBuilderImpl) buildUpdateRequest() (HTTPRequest, error) {
	path := fmt.Sprintf("/rest/v1/%s", url.PathEscape(q.table))

	// Build query string for filters
	params := url.Values{}
	for _, filter := range q.filters {
		parts := strings.SplitN(filter, "=", 2)
		if len(parts) == 2 {
			params.Add(parts[0], parts[1])
		}
	}

	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}

	body, err := json.Marshal(q.data)
	if err != nil {
		return HTTPRequest{}, fmt.Errorf("failed to marshal update data: %w", err)
	}

	return HTTPRequest{
		Method: "PATCH",
		URL:    path,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
			"Prefer":       "return=representation",
		},
		Body: body,
	}, nil
}

// buildDeleteRequest builds a DELETE request for DELETE operations.
func (q *queryBuilderImpl) buildDeleteRequest() (HTTPRequest, error) {
	path := fmt.Sprintf("/rest/v1/%s", url.PathEscape(q.table))

	// Build query string for filters
	params := url.Values{}
	for _, filter := range q.filters {
		parts := strings.SplitN(filter, "=", 2)
		if len(parts) == 2 {
			params.Add(parts[0], parts[1])
		}
	}

	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}

	return HTTPRequest{
		Method: "DELETE",
		URL:    path,
		Headers: map[string]string{
			"Accept": "application/json",
			"Prefer": "return=representation",
		},
	}, nil
}
