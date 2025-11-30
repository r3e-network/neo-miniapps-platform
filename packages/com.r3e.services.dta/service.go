package dta

import (
	"context"
	"fmt"
	"strings"

	
	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Service manages DTA products and orders.
type Service struct {
	framework.ServiceBase
	accounts AccountChecker
	wallets  WalletChecker
	store    Store
	log      *logger.Logger
	hooks    core.ObservationHooks
}

// Name returns the stable service identifier.
func (s *Service) Name() string { return "dta" }

// Domain reports the service domain.
func (s *Service) Domain() string { return "dta" }

// Manifest describes the service contract for the engine OS.
func (s *Service) Manifest() *framework.Manifest {
	return &framework.Manifest{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Description:  "DTA products and orders",
		Layer:        "service",
		DependsOn:    []string{"store", "svc-accounts"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore},
		Capabilities: []string{"dta"},
	}
}

// Descriptor advertises the service for system discovery.
func (s *Service) Descriptor() core.Descriptor { return s.Manifest().ToDescriptor() }

// Start/Stop/Ready are inherited from framework.ServiceBase.

// New constructs a DTA service.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("dta")
	}
	svc := &Service{accounts: accounts, store: store, log: log, hooks: core.NoopObservationHooks}
	svc.SetName(svc.Name())
	return svc
}

// WithWalletChecker injects a wallet checker for ownership validation.
func (s *Service) WithWalletChecker(w WalletChecker) {
	s.wallets = w
}

// WithObservationHooks configures callbacks for order creation observability.
func (s *Service) WithObservationHooks(h core.ObservationHooks) {
	s.hooks = core.NormalizeHooks(h)
}

// CreateProduct registers a product for an account.
func (s *Service) CreateProduct(ctx context.Context, product Product) (Product, error) {
	if err := s.accounts.AccountExists(ctx, product.AccountID); err != nil {
		return Product{}, err
	}
	if err := s.normalizeProduct(&product); err != nil {
		return Product{}, err
	}
	created, err := s.store.CreateProduct(ctx, product)
	if err != nil {
		return Product{}, err
	}
	s.log.WithField("product_id", created.ID).WithField("account_id", created.AccountID).Info("dta product created")
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
	updated, err := s.store.UpdateProduct(ctx, product)
	if err != nil {
		return Product{}, err
	}
	s.log.WithField("product_id", product.ID).WithField("account_id", product.AccountID).Info("dta product updated")
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
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListProducts(ctx, accountID)
}

// CreateOrder creates a subscription/redemption order.
func (s *Service) CreateOrder(ctx context.Context, accountID, productID string, typ OrderType, amount string, walletAddr string, metadata map[string]string) (Order, error) {
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
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
	finish := core.StartObservation(ctx, s.hooks, attrs)
	created, err := s.store.CreateOrder(ctx, order)
	if err != nil {
		finish(err)
		return Order{}, err
	}
	finish(nil)
	s.log.WithField("order_id", created.ID).WithField("product_id", product.ID).Info("dta order created")
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
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
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
