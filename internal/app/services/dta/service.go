package dta

import (
	"context"
	"fmt"
	"strings"

	core "github.com/R3E-Network/service_layer/internal/app/core/service"
	domaindta "github.com/R3E-Network/service_layer/internal/app/domain/dta"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service manages DTA products and orders.
type Service struct {
	base  *core.Base
	store storage.DTAStore
	log   *logger.Logger
	hooks core.ObservationHooks
}

// New constructs a DTA service.
func New(accounts storage.AccountStore, store storage.DTAStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("dta")
	}
	return &Service{base: core.NewBase(accounts), store: store, log: log, hooks: core.NoopObservationHooks}
}

// WithWorkspaceWallets injects wallet store enforcement for orders.
func (s *Service) WithWorkspaceWallets(store storage.WorkspaceWalletStore) {
	s.base.SetWallets(store)
}

// WithObservationHooks configures callbacks for order creation observability.
func (s *Service) WithObservationHooks(h core.ObservationHooks) {
	if h.OnStart == nil && h.OnComplete == nil {
		s.hooks = core.NoopObservationHooks
		return
	}
	s.hooks = h
}

// CreateProduct registers a product for an account.
func (s *Service) CreateProduct(ctx context.Context, product domaindta.Product) (domaindta.Product, error) {
	if err := s.base.EnsureAccount(ctx, product.AccountID); err != nil {
		return domaindta.Product{}, err
	}
	if err := s.normalizeProduct(&product); err != nil {
		return domaindta.Product{}, err
	}
	created, err := s.store.CreateProduct(ctx, product)
	if err != nil {
		return domaindta.Product{}, err
	}
	s.log.WithField("product_id", created.ID).WithField("account_id", created.AccountID).Info("dta product created")
	return created, nil
}

// UpdateProduct updates product fields.
func (s *Service) UpdateProduct(ctx context.Context, product domaindta.Product) (domaindta.Product, error) {
	stored, err := s.store.GetProduct(ctx, product.ID)
	if err != nil {
		return domaindta.Product{}, err
	}
	if stored.AccountID != product.AccountID {
		return domaindta.Product{}, fmt.Errorf("product %s does not belong to account %s", product.ID, product.AccountID)
	}
	product.AccountID = stored.AccountID
	if err := s.normalizeProduct(&product); err != nil {
		return domaindta.Product{}, err
	}
	updated, err := s.store.UpdateProduct(ctx, product)
	if err != nil {
		return domaindta.Product{}, err
	}
	s.log.WithField("product_id", product.ID).WithField("account_id", product.AccountID).Info("dta product updated")
	return updated, nil
}

// GetProduct fetches a product ensuring ownership.
func (s *Service) GetProduct(ctx context.Context, accountID, productID string) (domaindta.Product, error) {
	product, err := s.store.GetProduct(ctx, productID)
	if err != nil {
		return domaindta.Product{}, err
	}
	if product.AccountID != accountID {
		return domaindta.Product{}, fmt.Errorf("product %s does not belong to account %s", productID, accountID)
	}
	return product, nil
}

// ListProducts lists account products.
func (s *Service) ListProducts(ctx context.Context, accountID string) ([]domaindta.Product, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.store.ListProducts(ctx, accountID)
}

// CreateOrder creates a subscription/redemption order.
func (s *Service) CreateOrder(ctx context.Context, accountID, productID string, typ domaindta.OrderType, amount string, walletAddr string, metadata map[string]string) (domaindta.Order, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return domaindta.Order{}, err
	}
	product, err := s.GetProduct(ctx, accountID, productID)
	if err != nil {
		return domaindta.Order{}, err
	}
	typ = domaindta.OrderType(strings.ToLower(strings.TrimSpace(string(typ))))
	switch typ {
	case domaindta.OrderTypeSubscription, domaindta.OrderTypeRedemption:
	default:
		return domaindta.Order{}, fmt.Errorf("invalid order type %s", typ)
	}
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return domaindta.Order{}, fmt.Errorf("amount is required")
	}
	wallet := strings.ToLower(strings.TrimSpace(walletAddr))
	if wallet == "" {
		return domaindta.Order{}, fmt.Errorf("wallet_address is required")
	}
	if err := s.ensureWalletOwned(ctx, accountID, wallet); err != nil {
		return domaindta.Order{}, err
	}
	order := domaindta.Order{
		AccountID: accountID,
		ProductID: product.ID,
		Type:      typ,
		Amount:    amount,
		Wallet:    wallet,
		Status:    domaindta.OrderStatusPending,
		Metadata:  core.NormalizeMetadata(metadata),
	}
	attrs := map[string]string{"product_id": product.ID, "order_type": string(typ)}
	finish := core.StartObservation(ctx, s.hooks, attrs)
	created, err := s.store.CreateOrder(ctx, order)
	if err != nil {
		finish(err)
		return domaindta.Order{}, err
	}
	finish(nil)
	s.log.WithField("order_id", created.ID).WithField("product_id", product.ID).Info("dta order created")
	return created, nil
}

// GetOrder fetches an order.
func (s *Service) GetOrder(ctx context.Context, accountID, orderID string) (domaindta.Order, error) {
	order, err := s.store.GetOrder(ctx, orderID)
	if err != nil {
		return domaindta.Order{}, err
	}
	if order.AccountID != accountID {
		return domaindta.Order{}, fmt.Errorf("order %s does not belong to account %s", orderID, accountID)
	}
	return order, nil
}

// ListOrders lists recent orders.
func (s *Service) ListOrders(ctx context.Context, accountID string, limit int) ([]domaindta.Order, error) {
	if err := s.base.EnsureAccount(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListOrders(ctx, accountID, clamped)
}

// Descriptor advertises the service placement and capabilities.
func (s *Service) Descriptor() core.Descriptor {
	return core.Descriptor{
		Name:         "dta",
		Domain:       "dta",
		Layer:        core.LayerPlatform,
		Capabilities: []string{"orders", "subscriptions", "dispatch"},
	}
}

func (s *Service) normalizeProduct(product *domaindta.Product) error {
	product.Name = strings.TrimSpace(product.Name)
	product.Symbol = strings.ToUpper(strings.TrimSpace(product.Symbol))
	product.Type = strings.ToLower(strings.TrimSpace(product.Type))
	product.SettlementTerms = strings.TrimSpace(product.SettlementTerms)
	product.Metadata = core.NormalizeMetadata(product.Metadata)
	if product.Name == "" {
		return fmt.Errorf("name is required")
	}
	if product.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	status := domaindta.ProductStatus(strings.ToLower(strings.TrimSpace(string(product.Status))))
	if status == "" {
		status = domaindta.ProductStatusInactive
	}
	switch status {
	case domaindta.ProductStatusInactive, domaindta.ProductStatusActive, domaindta.ProductStatusSuspended:
		product.Status = status
	default:
		return fmt.Errorf("invalid status %s", status)
	}
	return nil
}

func (s *Service) ensureWalletOwned(ctx context.Context, accountID, wallet string) error {
	return s.base.EnsureSignersOwned(ctx, accountID, []string{wallet})
}
