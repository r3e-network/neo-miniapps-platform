package httpapi

import (
	"fmt"
	"net/http"

	"github.com/R3E-Network/service_layer/internal/domain/account"
	domainccip "github.com/R3E-Network/service_layer/internal/domain/ccip"
	domainlink "github.com/R3E-Network/service_layer/internal/domain/datalink"
)

func (h *handler) accountCCIP(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.CCIP == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("ccip service not configured"))
		return
	}
	if len(rest) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch rest[0] {
	case "lanes":
		h.accountCCIPLanes(w, r, accountID, rest[1:])
	case "messages":
		h.accountCCIPMessages(w, r, accountID, rest[1:])
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *handler) accountCCIPLanes(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	switch len(rest) {
	case 0:
		switch r.Method {
		case http.MethodGet:
			lanes, err := h.app.CCIP.ListLanes(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, lanes)
		case http.MethodPost:
			var payload struct {
				Name           string            `json:"name"`
				SourceChain    string            `json:"source_chain"`
				DestChain      string            `json:"dest_chain"`
				SignerSet      []string          `json:"signer_set"`
				AllowedTokens  []string          `json:"allowed_tokens"`
				DeliveryPolicy map[string]any    `json:"delivery_policy"`
				Metadata       map[string]string `json:"metadata"`
				Tags           []string          `json:"tags"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			lane := domainccip.Lane{
				AccountID:      accountID,
				Name:           payload.Name,
				SourceChain:    payload.SourceChain,
				DestChain:      payload.DestChain,
				SignerSet:      payload.SignerSet,
				AllowedTokens:  payload.AllowedTokens,
				DeliveryPolicy: payload.DeliveryPolicy,
				Metadata:       payload.Metadata,
				Tags:           payload.Tags,
			}
			created, err := h.app.CCIP.CreateLane(r.Context(), lane)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, created)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	default:
		laneID := rest[0]
		if len(rest) > 1 && rest[1] == "messages" {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var payload struct {
				Payload        map[string]any             `json:"payload"`
				TokenTransfers []domainccip.TokenTransfer `json:"token_transfers"`
				Metadata       map[string]string          `json:"metadata"`
				Tags           []string                   `json:"tags"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			msg, err := h.app.CCIP.SendMessage(r.Context(), accountID, laneID, payload.Payload, payload.TokenTransfers, payload.Metadata, payload.Tags)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, msg)
			return
		}
		switch r.Method {
		case http.MethodGet:
			lane, err := h.app.CCIP.GetLane(r.Context(), accountID, laneID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, lane)
		case http.MethodPut:
			var payload struct {
				Name           string            `json:"name"`
				SourceChain    string            `json:"source_chain"`
				DestChain      string            `json:"dest_chain"`
				SignerSet      []string          `json:"signer_set"`
				AllowedTokens  []string          `json:"allowed_tokens"`
				DeliveryPolicy map[string]any    `json:"delivery_policy"`
				Metadata       map[string]string `json:"metadata"`
				Tags           []string          `json:"tags"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			lane := domainccip.Lane{
				ID:             laneID,
				AccountID:      accountID,
				Name:           payload.Name,
				SourceChain:    payload.SourceChain,
				DestChain:      payload.DestChain,
				SignerSet:      payload.SignerSet,
				AllowedTokens:  payload.AllowedTokens,
				DeliveryPolicy: payload.DeliveryPolicy,
				Metadata:       payload.Metadata,
				Tags:           payload.Tags,
			}
			updated, err := h.app.CCIP.UpdateLane(r.Context(), lane)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, updated)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func (h *handler) accountCCIPMessages(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
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
		msgs, err := h.app.CCIP.ListMessages(r.Context(), accountID, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, msgs)
	default:
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		msgID := rest[0]
		msg, err := h.app.CCIP.GetMessage(r.Context(), accountID, msgID)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, msg)
	}
}

func (h *handler) accountDataLink(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.DataLink == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("datalink service not configured"))
		return
	}
	if len(rest) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	switch rest[0] {
	case "channels":
		h.accountDataLinkChannels(w, r, accountID, rest[1:])
	case "deliveries":
		h.accountDataLinkDeliveries(w, r, accountID, rest[1:])
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *handler) accountDataLinkChannels(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			channels, err := h.app.DataLink.ListChannels(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, channels)
		case http.MethodPost:
			var payload struct {
				Name      string            `json:"name"`
				Endpoint  string            `json:"endpoint"`
				AuthToken string            `json:"auth_token"`
				SignerSet []string          `json:"signer_set"`
				Status    string            `json:"status"`
				Metadata  map[string]string `json:"metadata"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if len(payload.SignerSet) == 0 {
				writeError(w, http.StatusBadRequest, fmt.Errorf("signer_set is required"))
				return
			}
			ch := domainlink.Channel{
				AccountID: accountID,
				Name:      payload.Name,
				Endpoint:  payload.Endpoint,
				AuthToken: payload.AuthToken,
				SignerSet: payload.SignerSet,
				Status:    domainlink.ChannelStatus(payload.Status),
				Metadata:  payload.Metadata,
			}
			created, err := h.app.DataLink.CreateChannel(r.Context(), ch)
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

	channelID := rest[0]
	if len(rest) == 1 {
		switch r.Method {
		case http.MethodGet:
			ch, err := h.app.DataLink.GetChannel(r.Context(), accountID, channelID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, ch)
		case http.MethodPut:
			var payload struct {
				Name      string            `json:"name"`
				Endpoint  string            `json:"endpoint"`
				AuthToken string            `json:"auth_token"`
				SignerSet []string          `json:"signer_set"`
				Status    string            `json:"status"`
				Metadata  map[string]string `json:"metadata"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if len(payload.SignerSet) == 0 {
				writeError(w, http.StatusBadRequest, fmt.Errorf("signer_set is required"))
				return
			}
			ch := domainlink.Channel{
				ID:        channelID,
				AccountID: accountID,
				Name:      payload.Name,
				Endpoint:  payload.Endpoint,
				AuthToken: payload.AuthToken,
				SignerSet: payload.SignerSet,
				Status:    domainlink.ChannelStatus(payload.Status),
				Metadata:  payload.Metadata,
			}
			updated, err := h.app.DataLink.UpdateChannel(r.Context(), ch)
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

	if rest[1] == "deliveries" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Payload  map[string]any    `json:"payload"`
			Metadata map[string]string `json:"metadata"`
		}
		if err := decodeJSON(r.Body, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		del, err := h.app.DataLink.CreateDelivery(r.Context(), accountID, channelID, payload.Payload, payload.Metadata)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, del)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

func (h *handler) accountDataLinkDeliveries(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
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
		deliveries, err := h.app.DataLink.ListDeliveries(r.Context(), accountID, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, deliveries)
	default:
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		deliveryID := rest[0]
		del, err := h.app.DataLink.GetDelivery(r.Context(), accountID, deliveryID)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, del)
	}
}

func (h *handler) accountWorkspaceWallets(w http.ResponseWriter, r *http.Request, accountID string, rest []string) {
	if h.app.WorkspaceWallets == nil {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("workspace wallet store not configured"))
		return
	}
	switch len(rest) {
	case 0:
		if r.Method == http.MethodGet {
			wallets, err := h.app.WorkspaceWallets.ListWorkspaceWallets(r.Context(), accountID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, wallets)
			return
		}
		if r.Method == http.MethodPost {
			var payload struct {
				WalletAddress string `json:"wallet_address"`
				Label         string `json:"label"`
				Status        string `json:"status"`
			}
			if err := decodeJSON(r.Body, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			if err := account.ValidateWalletAddress(payload.WalletAddress); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			addr := account.NormalizeWalletAddress(payload.WalletAddress)
			wallet, err := h.app.WorkspaceWallets.CreateWorkspaceWallet(r.Context(), account.WorkspaceWallet{
				WorkspaceID:   accountID,
				WalletAddress: addr,
				Label:         payload.Label,
				Status:        payload.Status,
			})
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusCreated, wallet)
			return
		}
	default:
		if r.Method == http.MethodGet {
			walletID := rest[0]
			wallet, err := h.app.WorkspaceWallets.GetWorkspaceWallet(r.Context(), walletID)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			if wallet.WorkspaceID != accountID {
				writeError(w, http.StatusNotFound, fmt.Errorf("wallet not found"))
				return
			}
			writeJSON(w, http.StatusOK, wallet)
			return
		}
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}
