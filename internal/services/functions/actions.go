package functions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domaindatastreams "github.com/R3E-Network/service_layer/internal/domain/datastreams"
	"github.com/R3E-Network/service_layer/internal/domain/function"
	"github.com/R3E-Network/service_layer/internal/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/domain/trigger"
	core "github.com/R3E-Network/service_layer/internal/services/core"
	randomsvc "github.com/R3E-Network/service_layer/internal/services/random"
)

func (s *Service) processActions(ctx context.Context, def function.Definition, actions []function.Action) ([]function.ActionResult, error) {
	results := make([]function.ActionResult, 0, len(actions))
	var firstErr error

	for _, action := range actions {
		res := function.ActionResult{
			Action: action,
			Status: function.ActionStatusSucceeded,
		}

		switch action.Type {
		case function.ActionTypeGasBankEnsureAccount:
			accountResult, err := s.handleGasBankEnsure(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = accountResult
			}
		case function.ActionTypeGasBankWithdraw:
			withdrawResult, err := s.handleGasBankWithdraw(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = withdrawResult
			}
		case function.ActionTypeGasBankBalance:
			balanceResult, err := s.handleGasBankBalance(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = balanceResult
			}
		case function.ActionTypeGasBankListTx:
			listResult, err := s.handleGasBankListTransactions(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = listResult
			}
		case function.ActionTypeOracleCreateRequest:
			oracleResult, err := s.handleOracleCreateRequest(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = oracleResult
			}
		case function.ActionTypePriceFeedSnapshot:
			priceResult, err := s.handlePriceFeedSnapshot(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = priceResult
			}
		case function.ActionTypeDataFeedSubmit:
			dfResult, err := s.handleDataFeedSubmit(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = dfResult
			}
		case function.ActionTypeDatastreamPublish:
			dsResult, err := s.handleDatastreamPublish(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = dsResult
			}
		case function.ActionTypeDatalinkDeliver:
			dlResult, err := s.handleDatalinkDelivery(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = dlResult
			}
		case function.ActionTypeRandomGenerate:
			randomResult, err := s.handleRandomGenerate(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = randomResult
			}
		case function.ActionTypeTriggerRegister:
			triggerResult, err := s.handleTriggerRegister(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = triggerResult
			}
		case function.ActionTypeAutomationSchedule:
			autoResult, err := s.handleAutomationSchedule(ctx, def, action.Params)
			if err != nil {
				res.Status = function.ActionStatusFailed
				res.Error = err.Error()
			} else {
				res.Result = autoResult
			}
		default:
			res.Status = function.ActionStatusFailed
			res.Error = fmt.Sprintf("unsupported action type %q", action.Type)
		}

		results = append(results, res)
		if res.Status == function.ActionStatusFailed && firstErr == nil {
			firstErr = fmt.Errorf("devpack action %s failed: %s", action.Type, res.Error)
		}
	}

	return results, firstErr
}

func (s *Service) handleGasBankEnsure(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.gasBank == nil {
		return nil, fmt.Errorf("gas bank: %w", errDependencyUnavailable)
	}

	wallet := stringParam(params, "wallet", "")
	acct, err := s.gasBank.EnsureAccount(ctx, def.AccountID, wallet)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"account": structToMap(acct),
	}, nil
}

func (s *Service) handleGasBankWithdraw(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.gasBank == nil {
		return nil, fmt.Errorf("gas bank: %w", errDependencyUnavailable)
	}

	gasAccountID := stringParam(params, "gasAccountId", "")
	wallet := stringParam(params, "wallet", "")
	if gasAccountID == "" && wallet == "" {
		return nil, errors.New("gasbank.withdraw requires gasAccountId or wallet")
	}
	if gasAccountID == "" {
		ensured, err := s.gasBank.EnsureAccount(ctx, def.AccountID, wallet)
		if err != nil {
			return nil, fmt.Errorf("ensure account: %w", err)
		}
		gasAccountID = ensured.ID
	} else {
		existing, err := s.gasBank.GetAccount(ctx, gasAccountID)
		if err != nil {
			return nil, err
		}
		if existing.AccountID != def.AccountID {
			return nil, fmt.Errorf("gas account %s does not belong to account %s", gasAccountID, def.AccountID)
		}
	}

	amount, err := floatParam(params, "amount")
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}
	if amount <= 0 {
		return nil, errors.New("amount must be greater than zero")
	}

	toAddress := stringParam(params, "to", "")
	updated, tx, err := s.gasBank.Withdraw(ctx, def.AccountID, gasAccountID, amount, toAddress)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"account":     structToMap(updated),
		"transaction": structToMap(tx),
	}, nil
}

func (s *Service) handleGasBankBalance(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.gasBank == nil {
		return nil, fmt.Errorf("gas bank: %w", errDependencyUnavailable)
	}
	gasAccountID := stringParam(params, "gasAccountId", "")
	wallet := stringParam(params, "wallet", "")
	if gasAccountID == "" && wallet == "" {
		return nil, errors.New("gasbank.balance requires gasAccountId or wallet")
	}
	var acct gasbank.Account
	var err error
	if gasAccountID != "" {
		acct, err = s.gasBank.GetAccount(ctx, gasAccountID)
		if err != nil {
			return nil, err
		}
		if acct.AccountID != def.AccountID {
			return nil, fmt.Errorf("gas account %s does not belong to account %s", gasAccountID, def.AccountID)
		}
	} else {
		acct, err = s.gasBank.EnsureAccount(ctx, def.AccountID, wallet)
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"account": structToMap(acct),
	}, nil
}

func (s *Service) handleGasBankListTransactions(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.gasBank == nil {
		return nil, fmt.Errorf("gas bank: %w", errDependencyUnavailable)
	}
	gasAccountID := stringParam(params, "gasAccountId", "")
	wallet := stringParam(params, "wallet", "")
	if gasAccountID == "" && wallet == "" {
		return nil, errors.New("gasbank.listTransactions requires gasAccountId or wallet")
	}
	var acct gasbank.Account
	var err error
	if gasAccountID != "" {
		acct, err = s.gasBank.GetAccount(ctx, gasAccountID)
		if err != nil {
			return nil, err
		}
		if acct.AccountID != def.AccountID {
			return nil, fmt.Errorf("gas account %s does not belong to account %s", gasAccountID, def.AccountID)
		}
	} else {
		acct, err = s.gasBank.EnsureAccount(ctx, def.AccountID, wallet)
		if err != nil {
			return nil, err
		}
	}
	status := stringParam(params, "status", "")
	txType := stringParam(params, "type", "")
	limit := intParam(params, "limit", core.DefaultListLimit)
	if limit <= 0 {
		limit = core.DefaultListLimit
	}
	txs, err := s.gasBank.ListTransactionsFiltered(ctx, acct.ID, txType, status, limit)
	if err != nil {
		return nil, err
	}
	serialized := make([]map[string]any, len(txs))
	for i, tx := range txs {
		serialized[i] = structToMap(tx)
	}
	return map[string]any{
		"transactions": serialized,
	}, nil
}

func (s *Service) handleOracleCreateRequest(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.oracle == nil {
		return nil, fmt.Errorf("oracle: %w", errDependencyUnavailable)
	}
	dataSourceID := stringParam(params, "dataSourceId", "")
	if dataSourceID == "" {
		return nil, errors.New("oracle.createRequest requires dataSourceId")
	}
	payloadStr, err := stringOrJSON(params["payload"])
	if err != nil {
		return nil, fmt.Errorf("payload: %w", err)
	}
	alternateSources := stringSliceParam(params, "alternateSourceIds")
	if len(alternateSources) > 0 {
		var payloadObj map[string]any
		if trimmed := strings.TrimSpace(payloadStr); trimmed != "" {
			_ = json.Unmarshal([]byte(trimmed), &payloadObj)
		}
		if payloadObj == nil {
			payloadObj = make(map[string]any)
		}
		payloadObj["alternate_source_ids"] = alternateSources
		if updated, err := json.Marshal(payloadObj); err == nil {
			payloadStr = string(updated)
		}
	}
	req, err := s.oracle.CreateRequest(ctx, def.AccountID, dataSourceID, payloadStr)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"request": structToMap(req),
	}, nil
}

func (s *Service) handlePriceFeedSnapshot(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.priceFeeds == nil {
		return nil, fmt.Errorf("pricefeed: %w", errDependencyUnavailable)
	}
	feedID := stringParam(params, "feedId", "")
	if feedID == "" {
		feedID = stringParam(params, "feed_id", "")
	}
	if feedID == "" {
		return nil, errors.New("pricefeed.recordSnapshot requires feedId")
	}
	price, err := floatParam(params, "price")
	if err != nil {
		return nil, fmt.Errorf("price: %w", err)
	}
	if price <= 0 {
		return nil, errors.New("price must be positive")
	}
	source := stringParam(params, "source", "")
	collectedAt := time.Now().UTC()
	if ts := stringParam(params, "collectedAt", stringParam(params, "collected_at", "")); ts != "" {
		parsed, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			return nil, fmt.Errorf("collected_at: %w", err)
		}
		collectedAt = parsed
	}

	snap, err := s.priceFeeds.RecordSnapshot(ctx, feedID, price, source, collectedAt)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"snapshot": structToMap(snap),
	}, nil
}

func (s *Service) handleDataFeedSubmit(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.dataFeeds == nil {
		return nil, fmt.Errorf("datafeeds: %w", errDependencyUnavailable)
	}
	feedID := stringParam(params, "feedId", stringParam(params, "feed_id", ""))
	if feedID == "" {
		return nil, errors.New("datafeeds.submitUpdate requires feedId")
	}
	roundID := int64(intParam(params, "roundId", intParam(params, "round_id", 0)))
	if roundID <= 0 {
		return nil, errors.New("roundId must be positive")
	}
	price := stringParam(params, "price", "")
	if price == "" {
		return nil, errors.New("price is required")
	}
	ts := time.Now().UTC()
	if tsStr := stringParam(params, "timestamp", ""); tsStr != "" {
		parsed, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			return nil, fmt.Errorf("timestamp: %w", err)
		}
		ts = parsed
	}
	signer := stringParam(params, "signer", "")
	signature := stringParam(params, "signature", "")
	meta, _ := mapStringStringParam(params, "metadata")

	update, err := s.dataFeeds.SubmitUpdate(ctx, def.AccountID, feedID, roundID, price, ts, signer, signature, meta)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"update": structToMap(update),
	}, nil
}

func (s *Service) handleRandomGenerate(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.random == nil {
		return nil, fmt.Errorf("random: %w", errDependencyUnavailable)
	}
	length := intParam(params, "length", 32)
	if length <= 0 {
		length = 32
	}
	requestID := stringParam(params, "requestId", stringParam(params, "request_id", ""))

	res, err := s.random.Generate(ctx, def.AccountID, length, requestID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"length":     res.Length,
		"value":      randomsvc.EncodeResult(res),
		"created_at": res.CreatedAt,
		"request_id": res.RequestID,
		"counter":    res.Counter,
		"signature":  base64.StdEncoding.EncodeToString(res.Signature),
		"public_key": base64.StdEncoding.EncodeToString(res.PublicKey),
	}, nil
}

func (s *Service) handleDatastreamPublish(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.dataStreams == nil {
		return nil, fmt.Errorf("datastreams: %w", errDependencyUnavailable)
	}
	streamID := stringParam(params, "streamId", stringParam(params, "stream_id", ""))
	if streamID == "" {
		return nil, errors.New("datastreams.publishFrame requires streamId")
	}
	seq := int64(intParam(params, "sequence", intParam(params, "seq", 0)))
	if seq <= 0 {
		return nil, errors.New("sequence must be positive")
	}
	payload := mapFromAny(params["payload"])
	latency := intParam(params, "latencyMs", 0)
	status := domaindatastreams.FrameStatus(stringParam(params, "status", ""))
	metadata, _ := mapStringStringParam(params, "metadata")

	frame, err := s.dataStreams.CreateFrame(ctx, def.AccountID, streamID, seq, payload, latency, status, metadata)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"frame": structToMap(frame),
	}, nil
}

func (s *Service) handleDatalinkDelivery(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.dataLink == nil {
		return nil, fmt.Errorf("datalink: %w", errDependencyUnavailable)
	}
	channelID := stringParam(params, "channelId", stringParam(params, "channel_id", ""))
	if channelID == "" {
		return nil, errors.New("datalink.createDelivery requires channelId")
	}
	payload := mapFromAny(params["payload"])
	metadata, _ := mapStringStringParam(params, "metadata")

	delivery, err := s.dataLink.CreateDelivery(ctx, def.AccountID, channelID, payload, metadata)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"delivery": structToMap(delivery),
	}, nil
}

func (s *Service) handleTriggerRegister(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.triggers == nil {
		return nil, fmt.Errorf("triggers: %w", errDependencyUnavailable)
	}

	triggerType := stringParam(params, "type", "")
	if triggerType == "" {
		return nil, errors.New("triggers.register requires type")
	}
	rule := stringParam(params, "rule", "")
	configMap, err := mapStringStringParam(params, "config")
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	enabled := boolParam(params, "enabled", true)

	trg := trigger.Trigger{
		AccountID:  def.AccountID,
		FunctionID: def.ID,
		Type:       trigger.Type(triggerType),
		Rule:       rule,
		Config:     configMap,
		Enabled:    enabled,
	}
	created, err := s.RegisterTrigger(ctx, trg)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"trigger": structToMap(created),
	}, nil
}

func (s *Service) handleAutomationSchedule(ctx context.Context, def function.Definition, params map[string]any) (map[string]any, error) {
	if s.automation == nil {
		return nil, fmt.Errorf("automation: %w", errDependencyUnavailable)
	}

	name := stringParam(params, "name", "")
	if name == "" {
		return nil, errors.New("automation.schedule requires name")
	}
	schedule := stringParam(params, "schedule", "")
	if schedule == "" {
		return nil, errors.New("automation.schedule requires schedule")
	}
	description := stringParam(params, "description", "")
	job, err := s.ScheduleAutomationJob(ctx, def.AccountID, def.ID, name, schedule, description)
	if err != nil {
		return nil, err
	}
	if _, exists := params["enabled"]; exists {
		if enabled := boolParam(params, "enabled", true); enabled != job.Enabled {
			if _, err := s.SetAutomationEnabled(ctx, job.ID, enabled); err != nil {
				return nil, fmt.Errorf("set automation enabled: %w", err)
			}
			job.Enabled = enabled
		}
	}
	return map[string]any{
		"job": structToMap(job),
	}, nil
}
