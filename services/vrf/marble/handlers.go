package neovrf

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/infrastructure/crypto"
	"github.com/R3E-Network/service_layer/infrastructure/httputil"
)

func (s *Service) registerRoutes() {
	s.Router().HandleFunc("/random", s.handleRandom).Methods(http.MethodPost)
	s.Router().HandleFunc("/pubkey", s.handlePubKey).Methods(http.MethodGet)
}

func (s *Service) handleRandom(w http.ResponseWriter, r *http.Request) {
	if _, ok := httputil.RequireUserID(w, r); !ok {
		return
	}

	var input RandomRequest
	if !httputil.DecodeJSONOptional(w, r, &input) {
		return
	}

	requestID := strings.TrimSpace(input.RequestID)
	if len(requestID) > 128 {
		httputil.BadRequest(w, "request_id too long")
		return
	}
	if requestID == "" {
		requestID = uuid.New().String()
	}

	if s.privateKey == nil {
		httputil.ServiceUnavailable(w, "signing key not configured")
		return
	}

	signature, err := crypto.Sign(s.privateKey, []byte(requestID))
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}

	randomness := crypto.Hash256(signature)

	resp := RandomResponse{
		RequestID:  requestID,
		Randomness: fmt.Sprintf("%x", randomness),
		Timestamp:  time.Now().Unix(),
	}
	if len(signature) > 0 {
		resp.Signature = fmt.Sprintf("%x", signature)
	}
	if len(s.publicKey) > 0 {
		resp.PublicKey = fmt.Sprintf("%x", s.publicKey)
	}
	if len(s.attestationHash) > 0 {
		resp.AttestationHash = fmt.Sprintf("%x", s.attestationHash)
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}

func (s *Service) handlePubKey(w http.ResponseWriter, r *http.Request) {
	if len(s.publicKey) == 0 {
		httputil.ServiceUnavailable(w, "public key not available")
		return
	}

	resp := PublicKeyResponse{
		PublicKey: fmt.Sprintf("%x", s.publicKey),
	}
	if len(s.attestationHash) > 0 {
		resp.AttestationHash = fmt.Sprintf("%x", s.attestationHash)
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}
