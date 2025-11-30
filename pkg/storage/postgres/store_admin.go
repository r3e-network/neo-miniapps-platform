package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/R3E-Network/service_layer/pkg/admin"
)

// AdminConfigStore implementation

// CreateChainRPC inserts a new chain RPC endpoint.
func (s *Store) CreateChainRPC(ctx context.Context, rpc admin.ChainRPC) (admin.ChainRPC, error) {
	if rpc.ID == "" {
		rpc.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	rpc.CreatedAt = now
	rpc.UpdatedAt = now

	metadataJSON, _ := json.Marshal(rpc.Metadata)
	if rpc.Metadata == nil {
		metadataJSON = []byte("{}")
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_chain_rpcs
		(id, chain_id, name, rpc_url, ws_url, chain_type, network_id, priority, weight, max_rps, timeout_ms, enabled, healthy, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, rpc.ID, rpc.ChainID, rpc.Name, rpc.RPCURL, rpc.WSURL, rpc.ChainType, rpc.NetworkID,
		rpc.Priority, rpc.Weight, rpc.MaxRPS, rpc.Timeout, rpc.Enabled, rpc.Healthy, metadataJSON, rpc.CreatedAt, rpc.UpdatedAt)
	if err != nil {
		return admin.ChainRPC{}, err
	}
	return rpc, nil
}

// UpdateChainRPC updates an existing chain RPC endpoint.
func (s *Store) UpdateChainRPC(ctx context.Context, rpc admin.ChainRPC) (admin.ChainRPC, error) {
	rpc.UpdatedAt = time.Now().UTC()

	metadataJSON, _ := json.Marshal(rpc.Metadata)
	if rpc.Metadata == nil {
		metadataJSON = []byte("{}")
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE admin_chain_rpcs SET
			chain_id = $2, name = $3, rpc_url = $4, ws_url = $5, chain_type = $6, network_id = $7,
			priority = $8, weight = $9, max_rps = $10, timeout_ms = $11, enabled = $12, healthy = $13,
			metadata = $14, updated_at = $15, last_check_at = $16
		WHERE id = $1
	`, rpc.ID, rpc.ChainID, rpc.Name, rpc.RPCURL, rpc.WSURL, rpc.ChainType, rpc.NetworkID,
		rpc.Priority, rpc.Weight, rpc.MaxRPS, rpc.Timeout, rpc.Enabled, rpc.Healthy,
		metadataJSON, rpc.UpdatedAt, toNullTime(rpc.LastCheckAt))
	if err != nil {
		return admin.ChainRPC{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return admin.ChainRPC{}, sql.ErrNoRows
	}
	return rpc, nil
}

// GetChainRPC retrieves a chain RPC by ID.
func (s *Store) GetChainRPC(ctx context.Context, id string) (admin.ChainRPC, error) {
	var rpc admin.ChainRPC
	var metadataJSON []byte
	var lastCheck sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, chain_id, name, rpc_url, ws_url, chain_type, network_id, priority, weight,
		       max_rps, timeout_ms, enabled, healthy, metadata, created_at, updated_at, last_check_at
		FROM admin_chain_rpcs WHERE id = $1
	`, id).Scan(&rpc.ID, &rpc.ChainID, &rpc.Name, &rpc.RPCURL, &rpc.WSURL, &rpc.ChainType,
		&rpc.NetworkID, &rpc.Priority, &rpc.Weight, &rpc.MaxRPS, &rpc.Timeout,
		&rpc.Enabled, &rpc.Healthy, &metadataJSON, &rpc.CreatedAt, &rpc.UpdatedAt, &lastCheck)
	if err != nil {
		return admin.ChainRPC{}, err
	}
	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &rpc.Metadata)
	}
	if lastCheck.Valid {
		rpc.LastCheckAt = lastCheck.Time
	}
	return rpc, nil
}

// GetChainRPCByChainID retrieves all RPC endpoints for a specific chain.
func (s *Store) GetChainRPCByChainID(ctx context.Context, chainID string) ([]admin.ChainRPC, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, chain_id, name, rpc_url, ws_url, chain_type, network_id, priority, weight,
		       max_rps, timeout_ms, enabled, healthy, metadata, created_at, updated_at, last_check_at
		FROM admin_chain_rpcs WHERE chain_id = $1 AND enabled = true
		ORDER BY priority ASC, weight DESC
	`, chainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChainRPCs(rows)
}

// ListChainRPCs lists all chain RPC endpoints.
func (s *Store) ListChainRPCs(ctx context.Context) ([]admin.ChainRPC, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, chain_id, name, rpc_url, ws_url, chain_type, network_id, priority, weight,
		       max_rps, timeout_ms, enabled, healthy, metadata, created_at, updated_at, last_check_at
		FROM admin_chain_rpcs ORDER BY chain_id, priority ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChainRPCs(rows)
}

// DeleteChainRPC deletes a chain RPC endpoint.
func (s *Store) DeleteChainRPC(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM admin_chain_rpcs WHERE id = $1`, id)
	return err
}

func scanChainRPCs(rows *sql.Rows) ([]admin.ChainRPC, error) {
	var result []admin.ChainRPC
	for rows.Next() {
		var rpc admin.ChainRPC
		var metadataJSON []byte
		var lastCheck sql.NullTime

		if err := rows.Scan(&rpc.ID, &rpc.ChainID, &rpc.Name, &rpc.RPCURL, &rpc.WSURL, &rpc.ChainType,
			&rpc.NetworkID, &rpc.Priority, &rpc.Weight, &rpc.MaxRPS, &rpc.Timeout,
			&rpc.Enabled, &rpc.Healthy, &metadataJSON, &rpc.CreatedAt, &rpc.UpdatedAt, &lastCheck); err != nil {
			return nil, err
		}
		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &rpc.Metadata)
		}
		if lastCheck.Valid {
			rpc.LastCheckAt = lastCheck.Time
		}
		result = append(result, rpc)
	}
	return result, rows.Err()
}

// CreateDataProvider inserts a new data provider.
func (s *Store) CreateDataProvider(ctx context.Context, p admin.DataProvider) (admin.DataProvider, error) {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now

	metadataJSON, _ := json.Marshal(p.Metadata)
	if p.Metadata == nil {
		metadataJSON = []byte("{}")
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_data_providers
		(id, name, type, base_url, api_key, rate_limit, timeout_ms, retries, enabled, healthy, features, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, p.ID, p.Name, p.Type, p.BaseURL, p.APIKey, p.RateLimit, p.Timeout, p.Retries,
		p.Enabled, p.Healthy, pq.Array(p.Features), metadataJSON, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return admin.DataProvider{}, err
	}
	return p, nil
}

// UpdateDataProvider updates an existing data provider.
func (s *Store) UpdateDataProvider(ctx context.Context, p admin.DataProvider) (admin.DataProvider, error) {
	p.UpdatedAt = time.Now().UTC()

	metadataJSON, _ := json.Marshal(p.Metadata)
	if p.Metadata == nil {
		metadataJSON = []byte("{}")
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE admin_data_providers SET
			name = $2, type = $3, base_url = $4, api_key = $5, rate_limit = $6, timeout_ms = $7,
			retries = $8, enabled = $9, healthy = $10, features = $11, metadata = $12,
			updated_at = $13, last_check_at = $14
		WHERE id = $1
	`, p.ID, p.Name, p.Type, p.BaseURL, p.APIKey, p.RateLimit, p.Timeout, p.Retries,
		p.Enabled, p.Healthy, pq.Array(p.Features), metadataJSON, p.UpdatedAt, toNullTime(p.LastCheckAt))
	if err != nil {
		return admin.DataProvider{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return admin.DataProvider{}, sql.ErrNoRows
	}
	return p, nil
}

// GetDataProvider retrieves a data provider by ID.
func (s *Store) GetDataProvider(ctx context.Context, id string) (admin.DataProvider, error) {
	var p admin.DataProvider
	var metadataJSON []byte
	var lastCheck sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, type, base_url, api_key, rate_limit, timeout_ms, retries, enabled, healthy,
		       features, metadata, created_at, updated_at, last_check_at
		FROM admin_data_providers WHERE id = $1
	`, id).Scan(&p.ID, &p.Name, &p.Type, &p.BaseURL, &p.APIKey, &p.RateLimit, &p.Timeout,
		&p.Retries, &p.Enabled, &p.Healthy, pq.Array(&p.Features), &metadataJSON,
		&p.CreatedAt, &p.UpdatedAt, &lastCheck)
	if err != nil {
		return admin.DataProvider{}, err
	}
	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &p.Metadata)
	}
	if lastCheck.Valid {
		p.LastCheckAt = lastCheck.Time
	}
	return p, nil
}

// ListDataProviders lists all data providers.
func (s *Store) ListDataProviders(ctx context.Context) ([]admin.DataProvider, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, type, base_url, api_key, rate_limit, timeout_ms, retries, enabled, healthy,
		       features, metadata, created_at, updated_at, last_check_at
		FROM admin_data_providers ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDataProviders(rows)
}

// ListDataProvidersByType lists data providers by type.
func (s *Store) ListDataProvidersByType(ctx context.Context, providerType string) ([]admin.DataProvider, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, type, base_url, api_key, rate_limit, timeout_ms, retries, enabled, healthy,
		       features, metadata, created_at, updated_at, last_check_at
		FROM admin_data_providers WHERE type = $1 ORDER BY name
	`, providerType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDataProviders(rows)
}

// DeleteDataProvider deletes a data provider.
func (s *Store) DeleteDataProvider(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM admin_data_providers WHERE id = $1`, id)
	return err
}

func scanDataProviders(rows *sql.Rows) ([]admin.DataProvider, error) {
	var result []admin.DataProvider
	for rows.Next() {
		var p admin.DataProvider
		var metadataJSON []byte
		var lastCheck sql.NullTime

		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.BaseURL, &p.APIKey, &p.RateLimit, &p.Timeout,
			&p.Retries, &p.Enabled, &p.Healthy, pq.Array(&p.Features), &metadataJSON,
			&p.CreatedAt, &p.UpdatedAt, &lastCheck); err != nil {
			return nil, err
		}
		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &p.Metadata)
		}
		if lastCheck.Valid {
			p.LastCheckAt = lastCheck.Time
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// GetSetting retrieves a system setting.
func (s *Store) GetSetting(ctx context.Context, key string) (admin.SystemSetting, error) {
	var setting admin.SystemSetting
	err := s.db.QueryRowContext(ctx, `
		SELECT key, value, type, category, description, editable, updated_at, updated_by
		FROM admin_settings WHERE key = $1
	`, key).Scan(&setting.Key, &setting.Value, &setting.Type, &setting.Category,
		&setting.Description, &setting.Editable, &setting.UpdatedAt, &setting.UpdatedBy)
	return setting, err
}

// SetSetting creates or updates a system setting.
func (s *Store) SetSetting(ctx context.Context, setting admin.SystemSetting) error {
	setting.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_settings (key, value, type, category, description, editable, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = $7, updated_by = $8
	`, setting.Key, setting.Value, setting.Type, setting.Category, setting.Description,
		setting.Editable, setting.UpdatedAt, setting.UpdatedBy)
	return err
}

// ListSettings lists system settings by category.
func (s *Store) ListSettings(ctx context.Context, category string) ([]admin.SystemSetting, error) {
	query := `SELECT key, value, type, category, description, editable, updated_at, updated_by FROM admin_settings`
	args := []interface{}{}
	if category != "" {
		query += ` WHERE category = $1`
		args = append(args, category)
	}
	query += ` ORDER BY category, key`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []admin.SystemSetting
	for rows.Next() {
		var setting admin.SystemSetting
		if err := rows.Scan(&setting.Key, &setting.Value, &setting.Type, &setting.Category,
			&setting.Description, &setting.Editable, &setting.UpdatedAt, &setting.UpdatedBy); err != nil {
			return nil, err
		}
		result = append(result, setting)
	}
	return result, rows.Err()
}

// DeleteSetting deletes a system setting.
func (s *Store) DeleteSetting(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM admin_settings WHERE key = $1`, key)
	return err
}

// GetFeatureFlag retrieves a feature flag.
func (s *Store) GetFeatureFlag(ctx context.Context, key string) (admin.FeatureFlag, error) {
	var flag admin.FeatureFlag
	err := s.db.QueryRowContext(ctx, `
		SELECT key, enabled, description, rollout, updated_at, updated_by
		FROM admin_feature_flags WHERE key = $1
	`, key).Scan(&flag.Key, &flag.Enabled, &flag.Description, &flag.Rollout, &flag.UpdatedAt, &flag.UpdatedBy)
	return flag, err
}

// SetFeatureFlag creates or updates a feature flag.
func (s *Store) SetFeatureFlag(ctx context.Context, flag admin.FeatureFlag) error {
	flag.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_feature_flags (key, enabled, description, rollout, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (key) DO UPDATE SET enabled = $2, description = $3, rollout = $4, updated_at = $5, updated_by = $6
	`, flag.Key, flag.Enabled, flag.Description, flag.Rollout, flag.UpdatedAt, flag.UpdatedBy)
	return err
}

// ListFeatureFlags lists all feature flags.
func (s *Store) ListFeatureFlags(ctx context.Context) ([]admin.FeatureFlag, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT key, enabled, description, rollout, updated_at, updated_by
		FROM admin_feature_flags ORDER BY key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []admin.FeatureFlag
	for rows.Next() {
		var flag admin.FeatureFlag
		if err := rows.Scan(&flag.Key, &flag.Enabled, &flag.Description, &flag.Rollout, &flag.UpdatedAt, &flag.UpdatedBy); err != nil {
			return nil, err
		}
		result = append(result, flag)
	}
	return result, rows.Err()
}

// GetTenantQuota retrieves tenant quota.
func (s *Store) GetTenantQuota(ctx context.Context, tenantID string) (admin.TenantQuota, error) {
	var quota admin.TenantQuota
	err := s.db.QueryRowContext(ctx, `
		SELECT tenant_id, max_accounts, max_functions, max_rpc_per_min, max_storage, max_gas_per_day, features, updated_at, updated_by
		FROM admin_tenant_quotas WHERE tenant_id = $1
	`, tenantID).Scan(&quota.TenantID, &quota.MaxAccounts, &quota.MaxFunctions, &quota.MaxRPCPerMin,
		&quota.MaxStorage, &quota.MaxGasPerDay, pq.Array(&quota.Features), &quota.UpdatedAt, &quota.UpdatedBy)
	return quota, err
}

// SetTenantQuota creates or updates tenant quota.
func (s *Store) SetTenantQuota(ctx context.Context, quota admin.TenantQuota) error {
	quota.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_tenant_quotas (tenant_id, max_accounts, max_functions, max_rpc_per_min, max_storage, max_gas_per_day, features, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id) DO UPDATE SET max_accounts = $2, max_functions = $3, max_rpc_per_min = $4,
			max_storage = $5, max_gas_per_day = $6, features = $7, updated_at = $8, updated_by = $9
	`, quota.TenantID, quota.MaxAccounts, quota.MaxFunctions, quota.MaxRPCPerMin,
		quota.MaxStorage, quota.MaxGasPerDay, pq.Array(quota.Features), quota.UpdatedAt, quota.UpdatedBy)
	return err
}

// ListTenantQuotas lists all tenant quotas.
func (s *Store) ListTenantQuotas(ctx context.Context) ([]admin.TenantQuota, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tenant_id, max_accounts, max_functions, max_rpc_per_min, max_storage, max_gas_per_day, features, updated_at, updated_by
		FROM admin_tenant_quotas ORDER BY tenant_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []admin.TenantQuota
	for rows.Next() {
		var quota admin.TenantQuota
		if err := rows.Scan(&quota.TenantID, &quota.MaxAccounts, &quota.MaxFunctions, &quota.MaxRPCPerMin,
			&quota.MaxStorage, &quota.MaxGasPerDay, pq.Array(&quota.Features), &quota.UpdatedAt, &quota.UpdatedBy); err != nil {
			return nil, err
		}
		result = append(result, quota)
	}
	return result, rows.Err()
}

// DeleteTenantQuota deletes tenant quota.
func (s *Store) DeleteTenantQuota(ctx context.Context, tenantID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM admin_tenant_quotas WHERE tenant_id = $1`, tenantID)
	return err
}

// GetAllowedMethods retrieves allowed methods for a chain.
func (s *Store) GetAllowedMethods(ctx context.Context, chainID string) (admin.AllowedMethod, error) {
	var methods admin.AllowedMethod
	err := s.db.QueryRowContext(ctx, `
		SELECT chain_id, methods FROM admin_allowed_methods WHERE chain_id = $1
	`, chainID).Scan(&methods.ChainID, pq.Array(&methods.Methods))
	return methods, err
}

// SetAllowedMethods creates or updates allowed methods for a chain.
func (s *Store) SetAllowedMethods(ctx context.Context, methods admin.AllowedMethod) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_allowed_methods (chain_id, methods, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (chain_id) DO UPDATE SET methods = $2, updated_at = NOW()
	`, methods.ChainID, pq.Array(methods.Methods))
	return err
}

// ListAllowedMethods lists all allowed methods configurations.
func (s *Store) ListAllowedMethods(ctx context.Context) ([]admin.AllowedMethod, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT chain_id, methods FROM admin_allowed_methods ORDER BY chain_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []admin.AllowedMethod
	for rows.Next() {
		var methods admin.AllowedMethod
		if err := rows.Scan(&methods.ChainID, pq.Array(&methods.Methods)); err != nil {
			return nil, err
		}
		result = append(result, methods)
	}
	return result, rows.Err()
}
