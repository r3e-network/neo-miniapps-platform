package dta

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/pkg/logger"
	
	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
	"github.com/R3E-Network/service_layer/system/os/sandbox"
)

// Service manages DTA products and orders.
type Service struct {
	*framework.SandboxedServiceEngine
	wallets WalletChecker
	store   Store
}

// New constructs a DTA service.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	return &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:         "dta",
				Description:  "DTA products and orders",
				DependsOn:    []string{"store", "svc-accounts"},
				RequiresAPIs: []framework.APISurface{framework.APISurfaceStore},
				Capabilities: []string{"dta"},
				Accounts:     accounts,
				Logger:       log,
			},
			SecurityLevel: sandbox.SecurityLevelPrivileged,
			RequestedCapabilities: []sandbox.Capability{
				sandbox.CapStorageRead,
				sandbox.CapStorageWrite,
				sandbox.CapDatabaseRead,
				sandbox.CapDatabaseWrite,
				sandbox.CapBusPublish,
				sandbox.CapServiceCall,
			},
			StorageQuota: 10 * 1024 * 1024,
		}),
		store: store,
	}
}

// WithWalletChecker injects a wallet checker for ownership validation.
func (s *Service) WithWalletChecker(w WalletChecker) {
	s.wallets = w
	s.SandboxedServiceEngine.WithWalletChecker(w)
}

// WithObservationHooks configures callbacks for order creation observability.
func (s *Service) WithObservationHooks(h core.ObservationHooks) {
	s.SandboxedServiceEngine.WithObservationHooks(h)
}

// CreateProduct registers a product for an account.
func (s *Service) CreateProduct(ctx context.Context, product Product) (Product, error) {
	if err := s.ValidateAccountExists(ctx, product.AccountID); err != nil {
		return Product{}, err
	}
	if err := s.normalizeProduct(&product); err != nil {
		return Product{}, err
	}
	attrs := map[string]string{"account_id": product.AccountID, "resource": "product"}
	ctx, finish := s.StartObservation(ctx, attrs)
	created, err := s.store.CreateProduct(ctx, product)
	if err == nil && created.ID != "" {
		attrs["product_id"] = created.ID
	}
	finish(err)
	if err != nil {
		return Product{}, err
	}
	s.Logger().WithField("product_id", created.ID).WithField("account_id", created.AccountID).Info("dta product created")
	s.LogCreated("dta_product", created.ID, created.AccountID)
	s.IncrementCounter("dta_products_created_total", map[string]string{"account_id": created.AccountID})
	return created, nil
}

// UpdateProduct updates product fields.
func (s *Service) UpdateProduct(ctx context.Context, product Product) (Product, error) {
	stored, err := s.store.GetProduct(ctx, product.ID)
	if err != nil {
		return Product{}, err
	}
	if err := core.EnsureOwnership(stored.AccountID, product.AccountID, "product", product.ID); err != nil {
		return Product{}, err
	}
	product.AccountID = stored.AccountID
	if err := s.normalizeProduct(&product); err != nil {
		return Product{}, err
	}
	attrs := map[string]string{"account_id": product.AccountID, "product_id": product.ID, "resource": "product"}
	ctx, finish := s.StartObservation(ctx, attrs)
	updated, err := s.store.UpdateProduct(ctx, product)
	finish(err)
	if err != nil {
		return Product{}, err
	}
	s.Logger().WithField("product_id", product.ID).WithField("account_id", product.AccountID).Info("dta product updated")
	s.LogUpdated("dta_product", product.ID, product.AccountID)
	s.IncrementCounter("dta_products_updated_total", map[string]string{"account_id": product.AccountID})
	return updated, nil
}

// GetProduct fetches a product ensuring ownership.
func (s *Service) GetProduct(ctx context.Context, accountID, productID string) (Product, error) {
	product, err := s.store.GetProduct(ctx, productID)
	if err != nil {
		return Product{}, err
	}
	if err := core.EnsureOwnership(product.AccountID, accountID, "product", productID); err != nil {
		return Product{}, err
	}
	return product, nil
}

// ListProducts lists account products.
func (s *Service) ListProducts(ctx context.Context, accountID string) ([]Product, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListProducts(ctx, accountID)
}

// CreateOrder creates a subscription/redemption order.
func (s *Service) CreateOrder(ctx context.Context, accountID, productID string, typ OrderType, amount string, walletAddr string, metadata map[string]string) (Order, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return Order{}, err
	}
	product, err := s.GetProduct(ctx, accountID, productID)
	if err != nil {
		return Order{}, err
	}
	typ = OrderType(strings.ToLower(strings.TrimSpace(string(typ))))
	switch typ {
	case OrderTypeSubscription, OrderTypeRedemption:
	default:
		return Order{}, fmt.Errorf("invalid order type %s", typ)
	}
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return Order{}, core.RequiredError("amount")
	}
	wallet := strings.ToLower(strings.TrimSpace(walletAddr))
	if wallet == "" {
		return Order{}, core.RequiredError("wallet_address")
	}
	if err := s.ensureWalletOwned(ctx, accountID, wallet); err != nil {
		return Order{}, err
	}
	order := Order{
		AccountID: accountID,
		ProductID: product.ID,
		Type:      typ,
		Amount:    amount,
		Wallet:    wallet,
		Status:    OrderStatusPending,
		Metadata:  core.NormalizeMetadata(metadata),
	}
	attrs := map[string]string{"product_id": product.ID, "order_type": string(typ)}
	ctx, finish := s.StartObservation(ctx, attrs)
	created, err := s.store.CreateOrder(ctx, order)
	if err == nil && created.ID != "" {
		attrs["order_id"] = created.ID
	}
	finish(err)
	if err != nil {
		return Order{}, err
	}
	s.Logger().WithField("order_id", created.ID).WithField("product_id", product.ID).Info("dta order created")
	s.LogCreated("dta_order", created.ID, accountID)
	s.IncrementCounter("dta_orders_created_total", map[string]string{"account_id": accountID, "product_id": product.ID})
	eventPayload := map[string]any{
		"order_id":   created.ID,
		"account_id": accountID,
		"product_id": product.ID,
		"type":       string(typ),
	}
	if err := s.PublishEvent(ctx, "dta.order.created", eventPayload); err != nil {
		if errors.Is(err, core.ErrBusUnavailable) {
			s.Logger().WithError(err).Warn("bus unavailable for dta.order.created event")
		} else {
			return Order{}, fmt.Errorf("publish order event: %w", err)
		}
	}
	return created, nil
}

// GetOrder fetches an order.
func (s *Service) GetOrder(ctx context.Context, accountID, orderID string) (Order, error) {
	order, err := s.store.GetOrder(ctx, orderID)
	if err != nil {
		return Order{}, err
	}
	if err := core.EnsureOwnership(order.AccountID, accountID, "order", orderID); err != nil {
		return Order{}, err
	}
	return order, nil
}

// ListOrders lists recent orders.
func (s *Service) ListOrders(ctx context.Context, accountID string, limit int) ([]Order, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListOrders(ctx, accountID, clamped)
}

func (s *Service) normalizeProduct(product *Product) error {
	product.Name = strings.TrimSpace(product.Name)
	product.Symbol = strings.ToUpper(strings.TrimSpace(product.Symbol))
	product.Type = strings.ToLower(strings.TrimSpace(product.Type))
	product.SettlementTerms = strings.TrimSpace(product.SettlementTerms)
	product.Metadata = core.NormalizeMetadata(product.Metadata)
	if product.Name == "" {
		return core.RequiredError("name")
	}
	if product.Symbol == "" {
		return core.RequiredError("symbol")
	}
	status := ProductStatus(strings.ToLower(strings.TrimSpace(string(product.Status))))
	if status == "" {
		status = ProductStatusInactive
	}
	switch status {
	case ProductStatusInactive, ProductStatusActive, ProductStatusSuspended:
		product.Status = status
	default:
		return fmt.Errorf("invalid status %s", status)
	}
	return nil
}

func (s *Service) ensureWalletOwned(ctx context.Context, accountID, wallet string) error {
	if wallet == "" {
		return core.RequiredError("wallet_address")
	}
	if s.wallets == nil {
		return nil
	}
	return s.wallets.WalletOwnedBy(ctx, accountID, wallet)
}

// HTTP API Methods
// These methods follow the HTTP{Method}{Path} naming convention for automatic route discovery.

// HTTPGetProducts handles GET /products - list all products for an account.
func (s *Service) HTTPGetProducts(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListProducts(ctx, req.AccountID)
}

// HTTPPostProducts handles POST /products - create a new product.
func (s *Service) HTTPPostProducts(ctx context.Context, req core.APIRequest) (any, error) {
	name, _ := req.Body["name"].(string)
	symbol, _ := req.Body["symbol"].(string)
	typ, _ := req.Body["type"].(string)
	settlementTerms, _ := req.Body["settlement_terms"].(string)
	status, _ := req.Body["status"].(string)

	metadata := core.ExtractMetadataRaw(req.Body, "")

	product := Product{
		AccountID:       req.AccountID,
		Name:            name,
		Symbol:          symbol,
		Type:            typ,
		SettlementTerms: settlementTerms,
		Status:          ProductStatus(status),
		Metadata:        metadata,
	}

	return s.CreateProduct(ctx, product)
}

// HTTPGetProductsById handles GET /products/{id} - get a specific product.
func (s *Service) HTTPGetProductsById(ctx context.Context, req core.APIRequest) (any, error) {
	productID := req.PathParams["id"]
	return s.GetProduct(ctx, req.AccountID, productID)
}

// HTTPPatchProductsById handles PATCH /products/{id} - update a product.
func (s *Service) HTTPPatchProductsById(ctx context.Context, req core.APIRequest) (any, error) {
	productID := req.PathParams["id"]

	// Get existing product first
	existing, err := s.GetProduct(ctx, req.AccountID, productID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if name, ok := req.Body["name"].(string); ok {
		existing.Name = name
	}
	if symbol, ok := req.Body["symbol"].(string); ok {
		existing.Symbol = symbol
	}
	if typ, ok := req.Body["type"].(string); ok {
		existing.Type = typ
	}
	if settlementTerms, ok := req.Body["settlement_terms"].(string); ok {
		existing.SettlementTerms = settlementTerms
	}
	if status, ok := req.Body["status"].(string); ok {
		existing.Status = ProductStatus(status)
	}

	existing.AccountID = req.AccountID
	return s.UpdateProduct(ctx, existing)
}

// HTTPGetOrders handles GET /orders - list all orders for an account.
func (s *Service) HTTPGetOrders(ctx context.Context, req core.APIRequest) (any, error) {
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListOrders(ctx, req.AccountID, limit)
}

// HTTPPostOrders handles POST /orders - create a new order.
func (s *Service) HTTPPostOrders(ctx context.Context, req core.APIRequest) (any, error) {
	productID, _ := req.Body["product_id"].(string)
	typ, _ := req.Body["type"].(string)
	amount, _ := req.Body["amount"].(string)
	walletAddr, _ := req.Body["wallet_address"].(string)

	metadata := core.ExtractMetadataRaw(req.Body, "")

	return s.CreateOrder(ctx, req.AccountID, productID, OrderType(typ), amount, walletAddr, metadata)
}

// HTTPGetOrdersById handles GET /orders/{id} - get a specific order.
func (s *Service) HTTPGetOrdersById(ctx context.Context, req core.APIRequest) (any, error) {
	orderID := req.PathParams["id"]
	return s.GetOrder(ctx, req.AccountID, orderID)
}
