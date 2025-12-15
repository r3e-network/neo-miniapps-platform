package neoaccountsmarble

import (
	"net/http"

	"github.com/R3E-Network/service_layer/internal/httputil"
)

func (s *Service) handleMasterKey(w http.ResponseWriter, r *http.Request) {
	att := s.buildMasterKeyAttestation()
	w.Header().Set("Cache-Control", "public, max-age=60")
	httputil.WriteJSON(w, http.StatusOK, att)
}
