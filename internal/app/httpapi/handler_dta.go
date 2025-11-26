package httpapi

import (
	"fmt"
	"net/http"

	domaindta "github.com/R3E-Network/service_layer/internal/domain/dta"
)

func (h *handler) accountDTA(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.DTA == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("dta service not configured"))
		return
	}
	if len(rest) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch rest[0] {
	case "products":
		h.accountDTAProducts(w, r, accountID, rest[1:])
	case "orders":
		h.accountDTAOrders(w, r, accountID, rest[1:])
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *handler) accountDTAProducts(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			products, err := h.app.DTA.ListProducts(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, products)
		case http.MethodPost:
			var payload struct {
				Name            string            `json:"name"`
				Symbol          string            `json:"symbol"`
				Type            string            `json:"type"`
				Status          string            `json:"status"`
				SettlementTerms string            `json:"settlement_terms"`
				Metadata        map[string]string `json:"metadata"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			product := domaindta.Product{
				AccountID:       accountID,
				Name:            payload.Name,
				Symbol:          payload.Symbol,
				Type:            payload.Type,
				Status:          domaindta.ProductStatus(payload.Status),
				SettlementTerms: payload.SettlementTerms,
				Metadata:        payload.Metadata,
			}
			created, err := h.app.DTA.CreateProduct(r.Context(), product)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, created)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	productID := rest[0]
	if len(rest) == 1 {
		switch r.Method {
		case http.MethodGet:
			product, err := h.app.DTA.GetProduct(r.Context(), accountID, productID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, product)
		case http.MethodPut:
			var payload struct {
				Name            string            `json:"name"`
				Symbol          string            `json:"symbol"`
				Type            string            `json:"type"`
				Status          string            `json:"status"`
				SettlementTerms string            `json:"settlement_terms"`
				Metadata        map[string]string `json:"metadata"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			product := domaindta.Product{
				ID:              productID,
				AccountID:       accountID,
				Name:            payload.Name,
				Symbol:          payload.Symbol,
				Type:            payload.Type,
				Status:          domaindta.ProductStatus(payload.Status),
				SettlementTerms: payload.SettlementTerms,
				Metadata:        payload.Metadata,
			}
			updated, err := h.app.DTA.UpdateProduct(r.Context(), product)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	if rest[1] == "orders" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Type          string            `json:"type"`
			Amount        string            `json:"amount"`
			WalletAddress string            `json:"wallet_address"`
			Metadata      map[string]string `json:"metadata"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		order, err := h.app.DTA.CreateOrder(r.Context(), accountID, productID, domaindta.OrderType(payload.Type), payload.Amount, payload.WalletAddress, payload.Metadata)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, order)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (h *handler) accountDTAOrders(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	switch len(rest) {
	case 0:
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		limit, err := parseLimitParam(r.URL.Query().Get("limit"), 25)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		orders, err := h.app.DTA.ListOrders(r.Context(), accountID, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, orders)
	default:
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		orderID := rest[0]
		order, err := h.app.DTA.GetOrder(r.Context(), accountID, orderID)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, order)
	}
}
