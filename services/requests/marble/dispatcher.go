package neorequests

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"github.com/R3E-Network/service_layer/infrastructure/chain"
	txproxytypes "github.com/R3E-Network/service_layer/infrastructure/txproxy/types"
	neorequestsupabase "github.com/R3E-Network/service_layer/services/requests/supabase"
)

type serviceResult struct {
	ResultBytes []byte
	AuditJSON   json.RawMessage
}

func (s *Service) handleServiceRequested(ctx context.Context, event *chain.ContractEvent) error {
	if event == nil {
		return nil
	}
	if s.serviceGatewayHash != "" && normalizeContractHash(event.Contract) != s.serviceGatewayHash {
		return nil
	}

	parsed, err := chain.ParseServiceRequestedEvent(event)
	if err != nil {
		return err
	}

	requestID := strings.TrimSpace(parsed.RequestID)
	appID := strings.TrimSpace(parsed.AppID)
	serviceType := normalizeServiceType(parsed.ServiceType)
	if requestID == "" || appID == "" || serviceType == "" {
		return fmt.Errorf("missing required request fields")
	}

	logger := s.Logger().WithFields(map[string]interface{}{
		"request_id":   requestID,
		"app_id":       appID,
		"service_type": serviceType,
	})

	if s.repo != nil {
		processed, err := s.markEventProcessed(ctx, event, parsed)
		if err != nil {
			logger.WithError(err).Warn("failed to mark event processed")
		}
		if !processed {
			return nil
		}
	}

	s.requestIndex.Store(requestID, appID)
	_ = s.storeContractEvent(ctx, event, &appID, buildServiceRequestedState(parsed))

	app, err := s.loadMiniApp(ctx, appID)
	if err != nil {
		logger.WithError(err).Warn("miniapp not found")
		return nil
	}
	if !isAppActive(app.Status) {
		logger.WithError(nil).Warn("miniapp disabled")
		serviceReq := s.createServiceRequest(ctx, app, parsed, serviceType)
		s.updateServiceRequest(ctx, serviceReq, nil, "failed", nil, "miniapp is not active")
		return nil
	}

	if err := s.validateAppRegistry(ctx, app); err != nil {
		logger.WithError(err).Warn("app registry validation failed")
		serviceReq := s.createServiceRequest(ctx, app, parsed, serviceType)
		s.updateServiceRequest(ctx, serviceReq, nil, "failed", nil, err.Error())
		return nil
	}

	manifestInfo, err := parseManifestInfo(app.Manifest)
	if err != nil {
		logger.WithError(err).Warn("invalid manifest")
		serviceReq := s.createServiceRequest(ctx, app, parsed, serviceType)
		s.updateServiceRequest(ctx, serviceReq, nil, "failed", nil, "invalid miniapp manifest")
		return nil
	}

	if !permissionEnabled(manifestInfo.Permissions, serviceTypePermission(serviceType)) {
		logger.WithError(nil).Warn("permission denied")
		serviceReq := s.createServiceRequest(ctx, app, parsed, serviceType)
		s.updateServiceRequest(ctx, serviceReq, nil, "failed", nil, "service permission not granted")
		return nil
	}

	if manifestInfo.CallbackContract != "" || manifestInfo.CallbackMethod != "" {
		if !callbackMatches(manifestInfo, parsed.CallbackContract, parsed.CallbackMethod) {
			logger.WithFields(map[string]interface{}{
				"manifest_callback_contract": manifestInfo.CallbackContract,
				"manifest_callback_method":   manifestInfo.CallbackMethod,
				"request_callback_contract":  parsed.CallbackContract,
				"request_callback_method":    parsed.CallbackMethod,
			}).Warn("callback target mismatch; skipping fulfillment")
			serviceReq := s.createServiceRequest(ctx, app, parsed, serviceType)
			s.updateServiceRequest(ctx, serviceReq, nil, "failed", nil, "callback target mismatch")
			return nil
		}
	}

	serviceReq := s.createServiceRequest(ctx, app, parsed, serviceType)

	result, execErr := s.executeService(ctx, app.DeveloperUserID, appID, requestID, serviceType, parsed.Payload)
	if execErr == nil && len(result.ResultBytes) > s.maxResult {
		execErr = fmt.Errorf("result exceeds max size")
	}

	success := execErr == nil
	fulfillErr := s.fulfillRequest(ctx, parsed, app.DeveloperUserID, result, execErr, serviceReq)
	if fulfillErr != nil {
		logger.WithError(fulfillErr).Warn("callback fulfillment failed")
	}

	if !success {
		logger.WithError(execErr).Warn("service execution failed")
	}

	return nil
}

func (s *Service) handleServiceFulfilled(ctx context.Context, event *chain.ContractEvent) error {
	if event == nil {
		return nil
	}
	if s.serviceGatewayHash != "" && normalizeContractHash(event.Contract) != s.serviceGatewayHash {
		return nil
	}

	parsed, err := chain.ParseServiceFulfilledEvent(event)
	if err != nil {
		return err
	}

	appID := ""
	if cached, ok := s.requestIndex.Load(strings.TrimSpace(parsed.RequestID)); ok {
		if value, ok := cached.(string); ok {
			appID = value
		}
	}

	var appPtr *string
	if appID != "" {
		appPtr = &appID
	}
	_ = s.storeContractEvent(ctx, event, appPtr, buildServiceFulfilledState(parsed))

	return nil
}

func (s *Service) executeService(ctx context.Context, userID, appID, requestID, serviceType string, payload []byte) (serviceResult, error) {
	switch serviceType {
	case "rng":
		return s.executeRNG(ctx, userID, appID, requestID, payload)
	case "oracle":
		return s.executeOracle(ctx, userID, payload)
	case "compute":
		return s.executeCompute(ctx, userID, payload)
	default:
		return serviceResult{}, fmt.Errorf("unsupported service type: %s", serviceType)
	}
}

func (s *Service) executeRNG(ctx context.Context, userID, appID, requestID string, payload []byte) (serviceResult, error) {
	if s.vrfURL == "" {
		return serviceResult{}, fmt.Errorf("neovrf URL not configured")
	}

	var req rngPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &req); err != nil {
			return serviceResult{}, fmt.Errorf("invalid rng payload")
		}
	}
	vrfRequestID := strings.TrimSpace(req.RequestID)
	if vrfRequestID == "" {
		vrfRequestID = fmt.Sprintf("%s:%s", appID, requestID)
	}

	respBytes, err := s.postJSON(ctx, joinURL(s.vrfURL, "/random"), userID, rngPayload{RequestID: vrfRequestID})
	if err != nil {
		return serviceResult{}, err
	}

	var resp rngResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return serviceResult{}, fmt.Errorf("invalid rng response")
	}

	audit := neorequestsupabase.MarshalParams(resp)
	if s.rngMode == "json" {
		return serviceResult{ResultBytes: respBytes, AuditJSON: audit}, nil
	}

	randomnessHex := strings.TrimPrefix(strings.TrimSpace(resp.Randomness), "0x")
	randomnessBytes, err := hex.DecodeString(randomnessHex)
	if err != nil || len(randomnessBytes) == 0 {
		return serviceResult{}, fmt.Errorf("invalid randomness payload")
	}

	return serviceResult{ResultBytes: randomnessBytes, AuditJSON: audit}, nil
}

func (s *Service) executeOracle(ctx context.Context, userID string, payload []byte) (serviceResult, error) {
	if s.oracleURL == "" {
		return serviceResult{}, fmt.Errorf("neooracle URL not configured")
	}
	if len(payload) == 0 {
		return serviceResult{}, fmt.Errorf("oracle payload required")
	}

	var req oraclePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return serviceResult{}, fmt.Errorf("invalid oracle payload")
	}
	if strings.TrimSpace(req.URL) == "" {
		return serviceResult{}, fmt.Errorf("oracle url required")
	}

	respBytes, err := s.postJSON(ctx, joinURL(s.oracleURL, "/query"), userID, req)
	if err != nil {
		return serviceResult{}, err
	}

	var resp oracleResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return serviceResult{}, fmt.Errorf("invalid oracle response")
	}

	var value gjson.Result
	if req.JSONPath != "" {
		value = gjson.Get(resp.Body, req.JSONPath)
		if !value.Exists() {
			return serviceResult{}, fmt.Errorf("json_path not found")
		}
	}

	result := map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Headers,
		"body":        resp.Body,
	}
	if req.JSONPath != "" {
		result = map[string]interface{}{
			"status_code": resp.StatusCode,
			"json_path":   req.JSONPath,
			"value":       value.Value(),
		}
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return serviceResult{}, fmt.Errorf("failed to marshal oracle result")
	}

	if s.maxResult > 0 && len(resultBytes) > s.maxResult {
		trimmed := map[string]interface{}{
			"status_code": resp.StatusCode,
		}
		if req.JSONPath != "" {
			trimmed["json_path"] = req.JSONPath
			trimmed["value"] = value.Value()
		} else if resp.Body != "" {
			trimmed["body"] = truncateString(resp.Body, s.maxResult/2)
		}
		resultBytes, err = json.Marshal(trimmed)
		if err != nil {
			return serviceResult{}, fmt.Errorf("failed to marshal trimmed oracle result")
		}
		if s.maxResult > 0 && len(resultBytes) > s.maxResult {
			return serviceResult{}, fmt.Errorf("oracle result exceeds max size")
		}
		result = trimmed
	}

	return serviceResult{ResultBytes: resultBytes, AuditJSON: neorequestsupabase.MarshalParams(result)}, nil
}

func (s *Service) executeCompute(ctx context.Context, userID string, payload []byte) (serviceResult, error) {
	if s.computeURL == "" {
		return serviceResult{}, fmt.Errorf("neocompute URL not configured")
	}
	if len(payload) == 0 {
		return serviceResult{}, fmt.Errorf("compute payload required")
	}

	var req computePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return serviceResult{}, fmt.Errorf("invalid compute payload")
	}
	if strings.TrimSpace(req.Script) == "" {
		return serviceResult{}, fmt.Errorf("compute script required")
	}
	if strings.TrimSpace(req.EntryPoint) == "" {
		req.EntryPoint = "main"
	}

	respBytes, err := s.postJSON(ctx, joinURL(s.computeURL, "/execute"), userID, req)
	if err != nil {
		return serviceResult{}, err
	}

	var resp computeResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return serviceResult{}, fmt.Errorf("invalid compute response")
	}

	if strings.ToLower(resp.Status) != "completed" {
		if resp.Error != "" {
			return serviceResult{}, errors.New(resp.Error)
		}
		return serviceResult{}, fmt.Errorf("compute failed")
	}

	result := map[string]interface{}{
		"job_id": resp.JobID,
		"status": resp.Status,
		"output": resp.Output,
	}
	if resp.Error != "" {
		result["error"] = resp.Error
	}
	if resp.OutputHash != "" {
		result["output_hash"] = resp.OutputHash
	}
	if resp.Signature != "" {
		result["signature"] = resp.Signature
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return serviceResult{}, fmt.Errorf("failed to marshal compute result")
	}

	if s.maxResult > 0 && len(resultBytes) > s.maxResult {
		trimmed := map[string]interface{}{
			"job_id": resp.JobID,
			"status": resp.Status,
		}
		if resp.OutputHash != "" {
			trimmed["output_hash"] = resp.OutputHash
		}
		if resp.Signature != "" {
			trimmed["signature"] = resp.Signature
		}
		if resp.Error != "" {
			trimmed["error"] = resp.Error
		}
		resultBytes, err = json.Marshal(trimmed)
		if err != nil {
			return serviceResult{}, fmt.Errorf("failed to marshal trimmed compute result")
		}
		if s.maxResult > 0 && len(resultBytes) > s.maxResult {
			return serviceResult{}, fmt.Errorf("compute result exceeds max size")
		}
		result = trimmed
	}

	return serviceResult{ResultBytes: resultBytes, AuditJSON: neorequestsupabase.MarshalParams(result)}, nil
}

func (s *Service) fulfillRequest(ctx context.Context, req *chain.ServiceRequestedEvent, userID string, result serviceResult, execErr error, serviceReq *neorequestsupabase.ServiceRequest) error {
	if s.txProxy == nil {
		return fmt.Errorf("txproxy not configured")
	}

	success := execErr == nil
	errorMsg := ""
	if execErr != nil {
		errorMsg = sanitizeError(execErr.Error(), s.maxErrorLen)
	}

	params, requestInt, err := buildFulfillParams(req.RequestID, success, result.ResultBytes, errorMsg)
	if err != nil {
		return err
	}

	requestKey := fmt.Sprintf("%s:%s:%s", ServiceID, req.AppID, req.RequestID)
	chainTx := &neorequestsupabase.ChainTx{
		RequestID:       requestKey,
		FromService:     ServiceID,
		TxType:          "service_callback",
		ContractAddress: "0x" + s.serviceGatewayHash,
		MethodName:      "fulfillRequest",
		Params:          neorequestsupabase.MarshalParams(params),
		Status:          "pending",
	}

	if s.repo != nil {
		if err := s.repo.CreateChainTx(ctx, chainTx); err != nil {
			s.Logger().WithContext(ctx).WithError(err).Warn("failed to create chain_txs row")
		} else if serviceReq != nil {
			serviceReq.ChainTxID = &chainTx.ID
			_ = s.repo.UpdateServiceRequest(ctx, serviceReq)
		}
	}

	resp, err := s.txProxy.Invoke(ctx, &txproxytypes.InvokeRequest{
		RequestID:    requestKey,
		ContractHash: "0x" + s.serviceGatewayHash,
		Method:       "fulfillRequest",
		Params:       params,
		Wait:         s.txWait,
	})
	if err != nil {
		if chainTx != nil {
			chainTx.Status = "failed"
			chainTx.ErrorMessage = sanitizeError(err.Error(), s.maxErrorLen)
			_ = s.updateChainTx(ctx, chainTx)
		}
		s.updateServiceRequest(ctx, serviceReq, nil, "failed", result.AuditJSON, err.Error())
		return err
	}

	status := "submitted"
	if s.txWait {
		if strings.EqualFold(resp.VMState, "HALT") {
			status = "confirmed"
		} else {
			status = "failed"
		}
	}

	if chainTx != nil {
		chainTx.TxHash = resp.TxHash
		chainTx.Status = status
		if resp.Exception != "" && status == "failed" {
			chainTx.ErrorMessage = sanitizeError(resp.Exception, s.maxErrorLen)
		}
		_ = s.updateChainTx(ctx, chainTx)
	}

	finalStatus := "completed"
	if !success || status == "failed" {
		finalStatus = "failed"
	}

	completedAt := time.Now().UTC()
	if serviceReq != nil {
		serviceReq.Status = finalStatus
		serviceReq.CompletedAt = &completedAt
		serviceReq.Result = result.AuditJSON
		if !success {
			serviceReq.Error = errorMsg
		}
		_ = s.repo.UpdateServiceRequest(ctx, serviceReq)
	}

	_ = requestInt
	return nil
}

func (s *Service) updateChainTx(ctx context.Context, chainTx *neorequestsupabase.ChainTx) error {
	if s.repo == nil || chainTx == nil || chainTx.ID == 0 {
		return nil
	}
	return s.repo.UpdateChainTx(ctx, chainTx)
}

func (s *Service) updateServiceRequest(ctx context.Context, req *neorequestsupabase.ServiceRequest, chainTxID *int64, status string, result json.RawMessage, errMsg string) {
	if s.repo == nil || req == nil {
		return
	}
	if chainTxID != nil {
		req.ChainTxID = chainTxID
	}
	if status != "" {
		req.Status = status
	}
	if len(result) > 0 {
		req.Result = result
	}
	if errMsg != "" {
		req.Error = sanitizeError(errMsg, s.maxErrorLen)
	}
	req.CompletedAt = ptrTime(time.Now().UTC())
	_ = s.repo.UpdateServiceRequest(ctx, req)
}

func (s *Service) createServiceRequest(ctx context.Context, app *neorequestsupabase.MiniApp, parsed *chain.ServiceRequestedEvent, serviceType string) *neorequestsupabase.ServiceRequest {
	if s.repo == nil || app == nil {
		return nil
	}

	payloadAudit := map[string]interface{}{
		"request_id":        parsed.RequestID,
		"app_id":            parsed.AppID,
		"service_type":      serviceType,
		"requester":         parsed.Requester,
		"callback_contract": parsed.CallbackContract,
		"callback_method":   parsed.CallbackMethod,
		"payload":           decodePayload(parsed.Payload),
	}

	req := &neorequestsupabase.ServiceRequest{
		UserID:      app.DeveloperUserID,
		ServiceType: serviceType,
		Status:      "processing",
		Payload:     neorequestsupabase.MarshalParams(payloadAudit),
	}

	if err := s.repo.CreateServiceRequest(ctx, req); err != nil {
		s.Logger().WithContext(ctx).WithError(err).Warn("failed to persist service request")
		return nil
	}
	return req
}

func (s *Service) loadMiniApp(ctx context.Context, appID string) (*neorequestsupabase.MiniApp, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}
	return s.repo.GetMiniApp(ctx, appID)
}

func (s *Service) markEventProcessed(ctx context.Context, event *chain.ContractEvent, parsed *chain.ServiceRequestedEvent) (bool, error) {
	if s.repo == nil || event == nil || parsed == nil {
		return true, nil
	}

	payload := map[string]interface{}{
		"request_id":        parsed.RequestID,
		"app_id":            parsed.AppID,
		"service_type":      parsed.ServiceType,
		"callback_contract": parsed.CallbackContract,
		"callback_method":   parsed.CallbackMethod,
	}

	processed := &neorequestsupabase.ProcessedEvent{
		ChainID:         s.chainID,
		TxHash:          event.TxHash,
		LogIndex:        event.LogIndex,
		BlockHeight:     event.BlockIndex,
		BlockHash:       event.BlockHash,
		ContractAddress: event.Contract,
		EventName:       event.EventName,
		Payload:         neorequestsupabase.MarshalParams(payload),
	}

	return s.repo.MarkProcessedEvent(ctx, processed)
}

func (s *Service) storeContractEvent(ctx context.Context, event *chain.ContractEvent, appID *string, state json.RawMessage) error {
	if s.repo == nil || event == nil {
		return nil
	}

	record := &neorequestsupabase.ContractEvent{
		TxHash:       event.TxHash,
		BlockIndex:   event.BlockIndex,
		ContractHash: event.Contract,
		EventName:    event.EventName,
		AppID:        appID,
		State:        state,
	}

	return s.repo.CreateContractEvent(ctx, record)
}

func buildFulfillParams(requestID string, success bool, result []byte, errorMsg string) ([]chain.ContractParam, *big.Int, error) {
	requestInt := new(big.Int)
	if _, ok := requestInt.SetString(strings.TrimSpace(requestID), 10); !ok {
		return nil, nil, fmt.Errorf("invalid request_id")
	}

	if result == nil {
		result = []byte{}
	}

	params := []chain.ContractParam{
		chain.NewIntegerParam(requestInt),
		chain.NewBoolParam(success),
		chain.NewByteArrayParam(result),
		chain.NewStringParam(errorMsg),
	}

	return params, requestInt, nil
}

func buildServiceRequestedState(event *chain.ServiceRequestedEvent) json.RawMessage {
	if event == nil {
		return nil
	}
	state := map[string]interface{}{
		"request_id":        event.RequestID,
		"app_id":            event.AppID,
		"service_type":      event.ServiceType,
		"requester":         event.Requester,
		"callback_contract": event.CallbackContract,
		"callback_method":   event.CallbackMethod,
		"payload":           decodePayload(event.Payload),
	}
	return neorequestsupabase.MarshalParams(state)
}

func buildServiceFulfilledState(event *chain.ServiceFulfilledEvent) json.RawMessage {
	if event == nil {
		return nil
	}
	state := map[string]interface{}{
		"request_id": event.RequestID,
		"success":    event.Success,
		"result":     decodeResult(event.Result),
		"error":      event.Error,
	}
	return neorequestsupabase.MarshalParams(state)
}

func decodePayload(payload []byte) interface{} {
	if len(payload) == 0 {
		return nil
	}
	var parsed interface{}
	if err := json.Unmarshal(payload, &parsed); err == nil {
		return parsed
	}
	return map[string]string{"base64": base64.StdEncoding.EncodeToString(payload)}
}

func decodeResult(result []byte) interface{} {
	if len(result) == 0 {
		return nil
	}
	var parsed interface{}
	if err := json.Unmarshal(result, &parsed); err == nil {
		return parsed
	}
	return map[string]string{"hex": hex.EncodeToString(result)}
}

func serviceTypePermission(serviceType string) string {
	switch serviceType {
	case "rng":
		return "rng"
	case "oracle":
		return "oracle"
	case "compute":
		return "compute"
	default:
		return serviceType
	}
}

func normalizeServiceType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "rng", "neovrf", "vrf":
		return "rng"
	case "oracle", "neooracle":
		return "oracle"
	case "compute", "neocompute", "confcompute":
		return "compute"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

type manifestInfo struct {
	CallbackContract string
	CallbackMethod   string
	Permissions      map[string]interface{}
}

func parseManifestInfo(raw json.RawMessage) (manifestInfo, error) {
	out := manifestInfo{Permissions: map[string]interface{}{}}
	if len(raw) == 0 {
		return out, nil
	}

	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return out, err
	}

	if val, ok := m["callback_contract"]; ok {
		contract := strings.TrimSpace(fmt.Sprintf("%v", val))
		if contract != "" {
			normalized := normalizeContractHash(contract)
			if normalized == "" {
				return out, fmt.Errorf("invalid callback_contract")
			}
			out.CallbackContract = "0x" + normalized
		}
	}
	if val, ok := m["callback_method"]; ok {
		out.CallbackMethod = strings.TrimSpace(fmt.Sprintf("%v", val))
	}

	if perms, ok := m["permissions"]; ok {
		switch v := perms.(type) {
		case map[string]interface{}:
			out.Permissions = v
		case []interface{}:
			for _, entry := range v {
				key := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", entry)))
				if key != "" {
					out.Permissions[key] = true
				}
			}
		}
	}

	return out, nil
}

func permissionEnabled(perms map[string]interface{}, key string) bool {
	if len(perms) == 0 || key == "" {
		return false
	}
	value, ok := perms[key]
	if !ok {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case []interface{}:
		return len(v) > 0
	default:
		return false
	}
}

func callbackMatches(info manifestInfo, contract, method string) bool {
	if info.CallbackContract == "" && info.CallbackMethod == "" {
		return true
	}
	if info.CallbackMethod != "" && info.CallbackMethod != strings.TrimSpace(method) {
		return false
	}
	if info.CallbackContract != "" {
		if normalizeContractHash(info.CallbackContract) != normalizeContractHash(contract) {
			return false
		}
	}
	return true
}

func isAppActive(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "active")
}

func sanitizeError(msg string, limit int) string {
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.TrimSpace(msg)
	if limit <= 0 || len(msg) <= limit {
		return msg
	}
	return msg[:limit]
}

func truncateString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func (s *Service) fulfillFailure(ctx context.Context, req *chain.ServiceRequestedEvent, userID string, err error) error {
	result := serviceResult{}
	return s.fulfillRequest(ctx, req, userID, result, err, nil)
}

func (s *Service) handleNotificationEvent(ctx context.Context, event *chain.ContractEvent) error {
	if event == nil {
		return nil
	}

	parsed, err := chain.ParseMiniAppNotificationEvent(event)
	if err != nil {
		return nil // Skip non-notification events
	}

	logger := s.Logger().WithFields(map[string]interface{}{
		"app_id": parsed.AppID,
		"title":  parsed.Title,
	})

	// Store notification in database via repository
	err = s.repo.CreateNotification(ctx, &neorequestsupabase.Notification{
		AppID:            parsed.AppID,
		Title:            parsed.Title,
		Content:          parsed.Content,
		NotificationType: parsed.NotificationType,
		Source:           "contract",
		TxHash:           event.TxHash,
		BlockNumber:      int64(event.BlockIndex),
		Priority:         parsed.Priority,
	})

	if err != nil {
		logger.WithError(err).Error("failed to store notification")
		return err
	}

	logger.Info("notification stored from contract event")
	return nil
}
