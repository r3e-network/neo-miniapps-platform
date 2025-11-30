package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/admin"
	"github.com/R3E-Network/service_layer/pkg/storage"
)

// adminConfigStore provides access to admin config storage.
type adminConfigStore interface {
	storage.AdminConfigStore
}

// AdminConfigHandler handles admin configuration API endpoints.
type AdminConfigHandler struct {
	store adminConfigStore
}

// NewAdminConfigHandler creates a new admin config handler.
func NewAdminConfigHandler(store adminConfigStore) *AdminConfigHandler {
	return &AdminConfigHandler{store: store}
}

// RegisterRoutes mounts admin config routes on the given mux.
func (h *AdminConfigHandler) RegisterRoutes(mux *http.ServeMux) {
	// Chain RPCs
	mux.HandleFunc("/admin/config/chains", h.handleChains)
	mux.HandleFunc("/admin/config/chains/", h.handleChainByID)

	// Data Providers
	mux.HandleFunc("/admin/config/providers", h.handleProviders)
	mux.HandleFunc("/admin/config/providers/", h.handleProviderByID)

	// System Settings
	mux.HandleFunc("/admin/config/settings", h.handleSettings)
	mux.HandleFunc("/admin/config/settings/", h.handleSettingByKey)

	// Feature Flags
	mux.HandleFunc("/admin/config/features", h.handleFeatures)
	mux.HandleFunc("/admin/config/features/", h.handleFeatureByKey)

	// Tenant Quotas
	mux.HandleFunc("/admin/config/quotas", h.handleQuotas)
	mux.HandleFunc("/admin/config/quotas/", h.handleQuotaByTenant)

	// Allowed Methods
	mux.HandleFunc("/admin/config/methods", h.handleMethods)
	mux.HandleFunc("/admin/config/methods/", h.handleMethodsByChain)
}

// ============================================================================
// Chain RPCs
// ============================================================================

func (h *AdminConfigHandler) handleChains(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		chains, err := h.store.ListChainRPCs(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if chains == nil {
			chains = []admin.ChainRPC{}
		}
		writeJSON(w, http.StatusOK, chains)

	case http.MethodPost:
		var rpc admin.ChainRPC
		if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if rpc.ChainID == "" || rpc.RPCURL == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("chain_id and rpc_url are required"))
			return
		}
		if rpc.Name == "" {
			rpc.Name = rpc.ChainID
		}
		if rpc.ChainType == "" {
			rpc.ChainType = "evm"
		}
		rpc.Enabled = true
		rpc.Healthy = true

		created, err := h.store.CreateChainRPC(r.Context(), rpc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *AdminConfigHandler) handleChainByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/admin/config/chains/")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("chain id required"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		rpc, err := h.store.GetChainRPC(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, rpc)

	case http.MethodPut, http.MethodPatch:
		var rpc admin.ChainRPC
		if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rpc.ID = id
		updated, err := h.store.UpdateChainRPC(r.Context(), rpc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		if err := h.store.DeleteChainRPC(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete)
	}
}

// ============================================================================
// Data Providers
// ============================================================================

func (h *AdminConfigHandler) handleProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		providerType := r.URL.Query().Get("type")
		var providers []admin.DataProvider
		var err error
		if providerType != "" {
			providers, err = h.store.ListDataProvidersByType(r.Context(), providerType)
		} else {
			providers, err = h.store.ListDataProviders(r.Context())
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if providers == nil {
			providers = []admin.DataProvider{}
		}
		writeJSON(w, http.StatusOK, providers)

	case http.MethodPost:
		var provider admin.DataProvider
		if err := json.NewDecoder(r.Body).Decode(&provider); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if provider.Name == "" || provider.BaseURL == "" || provider.Type == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("name, type, and base_url are required"))
			return
		}
		provider.Enabled = true
		provider.Healthy = true

		created, err := h.store.CreateDataProvider(r.Context(), provider)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *AdminConfigHandler) handleProviderByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/admin/config/providers/")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("provider id required"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		provider, err := h.store.GetDataProvider(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, provider)

	case http.MethodPut, http.MethodPatch:
		var provider admin.DataProvider
		if err := json.NewDecoder(r.Body).Decode(&provider); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		provider.ID = id
		updated, err := h.store.UpdateDataProvider(r.Context(), provider)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		if err := h.store.DeleteDataProvider(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete)
	}
}

// ============================================================================
// System Settings
// ============================================================================

func (h *AdminConfigHandler) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		category := r.URL.Query().Get("category")
		settings, err := h.store.ListSettings(r.Context(), category)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if settings == nil {
			settings = []admin.SystemSetting{}
		}
		writeJSON(w, http.StatusOK, settings)

	case http.MethodPost:
		var setting admin.SystemSetting
		if err := json.NewDecoder(r.Body).Decode(&setting); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if setting.Key == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("key is required"))
			return
		}
		if setting.Type == "" {
			setting.Type = "string"
		}
		if setting.Category == "" {
			setting.Category = "general"
		}
		setting.UpdatedBy = userFromCtx(r.Context())

		if err := h.store.SetSetting(r.Context(), setting); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, setting)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *AdminConfigHandler) handleSettingByKey(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/admin/config/settings/")
	if key == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("setting key required"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		setting, err := h.store.GetSetting(r.Context(), key)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, setting)

	case http.MethodPut, http.MethodPatch:
		var setting admin.SystemSetting
		if err := json.NewDecoder(r.Body).Decode(&setting); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		setting.Key = key
		setting.UpdatedBy = userFromCtx(r.Context())

		if err := h.store.SetSetting(r.Context(), setting); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, setting)

	case http.MethodDelete:
		if err := h.store.DeleteSetting(r.Context(), key); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete)
	}
}

// ============================================================================
// Feature Flags
// ============================================================================

func (h *AdminConfigHandler) handleFeatures(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		flags, err := h.store.ListFeatureFlags(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if flags == nil {
			flags = []admin.FeatureFlag{}
		}
		writeJSON(w, http.StatusOK, flags)

	case http.MethodPost:
		var flag admin.FeatureFlag
		if err := json.NewDecoder(r.Body).Decode(&flag); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if flag.Key == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("key is required"))
			return
		}
		if flag.Rollout == 0 && flag.Enabled {
			flag.Rollout = 100
		}
		flag.UpdatedBy = userFromCtx(r.Context())

		if err := h.store.SetFeatureFlag(r.Context(), flag); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, flag)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *AdminConfigHandler) handleFeatureByKey(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/admin/config/features/")
	if key == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("feature key required"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		flag, err := h.store.GetFeatureFlag(r.Context(), key)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, flag)

	case http.MethodPut, http.MethodPatch:
		var flag admin.FeatureFlag
		if err := json.NewDecoder(r.Body).Decode(&flag); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		flag.Key = key
		flag.UpdatedBy = userFromCtx(r.Context())

		if err := h.store.SetFeatureFlag(r.Context(), flag); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, flag)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodPatch)
	}
}

// ============================================================================
// Tenant Quotas
// ============================================================================

func (h *AdminConfigHandler) handleQuotas(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		quotas, err := h.store.ListTenantQuotas(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if quotas == nil {
			quotas = []admin.TenantQuota{}
		}
		writeJSON(w, http.StatusOK, quotas)

	case http.MethodPost:
		var quota admin.TenantQuota
		if err := json.NewDecoder(r.Body).Decode(&quota); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if quota.TenantID == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("tenant_id is required"))
			return
		}
		quota.UpdatedBy = userFromCtx(r.Context())

		if err := h.store.SetTenantQuota(r.Context(), quota); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, quota)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *AdminConfigHandler) handleQuotaByTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimPrefix(r.URL.Path, "/admin/config/quotas/")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("tenant id required"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		quota, err := h.store.GetTenantQuota(r.Context(), tenantID)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, quota)

	case http.MethodPut, http.MethodPatch:
		var quota admin.TenantQuota
		if err := json.NewDecoder(r.Body).Decode(&quota); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		quota.TenantID = tenantID
		quota.UpdatedBy = userFromCtx(r.Context())

		if err := h.store.SetTenantQuota(r.Context(), quota); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, quota)

	case http.MethodDelete:
		if err := h.store.DeleteTenantQuota(r.Context(), tenantID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete)
	}
}

// ============================================================================
// Allowed Methods
// ============================================================================

func (h *AdminConfigHandler) handleMethods(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		methods, err := h.store.ListAllowedMethods(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if methods == nil {
			methods = []admin.AllowedMethod{}
		}
		writeJSON(w, http.StatusOK, methods)

	case http.MethodPost:
		var methods admin.AllowedMethod
		if err := json.NewDecoder(r.Body).Decode(&methods); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if methods.ChainID == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("chain_id is required"))
			return
		}

		if err := h.store.SetAllowedMethods(r.Context(), methods); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, methods)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *AdminConfigHandler) handleMethodsByChain(w http.ResponseWriter, r *http.Request) {
	chainID := strings.TrimPrefix(r.URL.Path, "/admin/config/methods/")
	if chainID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("chain id required"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		methods, err := h.store.GetAllowedMethods(r.Context(), chainID)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, methods)

	case http.MethodPut, http.MethodPatch:
		var methods admin.AllowedMethod
		if err := json.NewDecoder(r.Body).Decode(&methods); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		methods.ChainID = chainID

		if err := h.store.SetAllowedMethods(r.Context(), methods); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, methods)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodPatch)
	}
}

// userFromCtx extracts the current user from context.
func userFromCtx(ctx context.Context) string {
	if u := ctx.Value(ctxUserKey); u != nil {
		if str, ok := u.(string); ok {
			return str
		}
	}
	return "system"
}
