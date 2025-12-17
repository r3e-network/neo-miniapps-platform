package txproxy

import (
	"net/http"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"

	"github.com/R3E-Network/service_layer/infrastructure/httputil"
)

func (s *Service) handleInvoke(w http.ResponseWriter, r *http.Request) {
	var req InvokeRequest
	if !httputil.DecodeJSON(w, r, &req) {
		return
	}

	reqID := strings.TrimSpace(req.RequestID)
	if reqID == "" {
		httputil.BadRequest(w, "request_id required")
		return
	}
	if !s.markSeen(reqID) {
		httputil.WriteError(w, http.StatusConflict, "request_id already used")
		return
	}

	contractHash := strings.TrimSpace(req.ContractHash)
	method := canonicalizeMethodName(req.Method)
	if contractHash == "" || method == "" {
		httputil.BadRequest(w, "contract_hash and method required")
		return
	}

	if s.allowlist == nil || !s.allowlist.Allows(contractHash, method) {
		httputil.WriteError(w, http.StatusForbidden, "contract/method not allowed")
		return
	}

	if status, msg := s.checkIntentPolicy(contractHash, method, req.Intent); status != 0 {
		httputil.WriteError(w, status, msg)
		return
	}

	if s.chainClient == nil || s.signer == nil {
		httputil.WriteError(w, http.StatusServiceUnavailable, "chain signing is not configured")
		return
	}

	txRes, err := s.chainClient.InvokeFunctionWithSignerAndWait(
		r.Context(),
		normalizeContractHash(contractHash),
		method,
		req.Params,
		s.signer,
		transaction.CalledByEntry,
		req.Wait,
	)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}

	resp := InvokeResponse{
		RequestID: reqID,
	}
	if txRes != nil {
		resp.TxHash = txRes.TxHash
		resp.VMState = txRes.VMState
		if txRes.AppLog != nil && len(txRes.AppLog.Executions) > 0 {
			resp.Exception = txRes.AppLog.Executions[0].Exception
		}
	}

	httputil.WriteJSON(w, http.StatusOK, resp)
}
